package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/alexfsong/jeeves/internal/llm"
	"github.com/alexfsong/jeeves/internal/resolution"
	"github.com/alexfsong/jeeves/internal/source"
	"github.com/alexfsong/jeeves/internal/store"
)

type Engine struct {
	search  source.Source
	wiki    *source.WikiSource
	fetcher *source.Fetcher
	router  *llm.Router
	store   *store.Store
}

type ResearchResult struct {
	Query      string          `json:"query"`
	Resolution string          `json:"resolution"`
	SubQueries []string        `json:"sub_queries,omitempty"`
	Results    []source.Result `json:"results"`
	Synthesis  string          `json:"synthesis,omitempty"`
	Warnings   []string        `json:"warnings,omitempty"`
	Persisted  int             `json:"persisted,omitempty"`
}

func New(search source.Source, wiki *source.WikiSource, fetcher *source.Fetcher, router *llm.Router, st *store.Store) *Engine {
	return &Engine{
		search:  search,
		wiki:    wiki,
		fetcher: fetcher,
		router:  router,
		store:   st,
	}
}

func (e *Engine) Research(ctx context.Context, query string, res resolution.Level, topicID *int64) (*ResearchResult, error) {
	result := &ResearchResult{
		Query:      query,
		Resolution: res.String(),
	}

	// Check local knowledge first (anticipatory)
	if e.wiki != nil {
		local, err := e.wiki.Search(ctx, query, 5)
		if err == nil && len(local) > 0 {
			result.Results = append(result.Results, local...)
		}
	}

	// Decompose query
	subQueries := Decompose(ctx, query, res, e.router)
	result.SubQueries = subQueries

	// Search
	var allResults []source.Result
	limit := res.MaxURLs()
	if limit == 0 {
		limit = 10 // glance still searches, just doesn't fetch
	}

	for _, sq := range subQueries {
		results, err := e.search.Search(ctx, sq, limit)
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("I regret the web search service is presently unavailable, sir. %v", err))
			continue
		}
		allResults = append(allResults, results...)
	}

	// Score and rank
	ScoreTFIDF(allResults, query)

	// Apply trusted source boosts
	if e.store != nil {
		trusted, err := e.store.ListTrustedSources(topicID)
		if err == nil && len(trusted) > 0 {
			ApplyTrustBoost(allResults, trusted)
		}
	}

	allResults = RankResults(allResults)

	// Merge with local results
	result.Results = RankResults(append(result.Results, allResults...))

	if res == resolution.Glance {
		// Glance: return snippets only
		return result, nil
	}

	// Brief+: Fetch URL content
	maxFetch := res.MaxURLs()
	if maxFetch > len(result.Results) {
		maxFetch = len(result.Results)
	}

	if maxFetch > 0 && e.fetcher != nil {
		e.fetchContent(ctx, result.Results[:maxFetch])
	}

	// Brief: local LLM summarize
	if res == resolution.Brief {
		if !e.router.LocalAvailable() {
			result.Warnings = append(result.Warnings,
				"Local LLM unavailable, sir. Falling back to glance resolution.")
			return result, nil
		}
		synthesis, err := e.summarize(ctx, query, result.Results[:maxFetch])
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Summarization encountered difficulties, sir: %v", err))
		} else {
			result.Synthesis = synthesis
		}
		return result, nil
	}

	// Detailed/Full: Cloud synthesis
	if !e.router.CloudAvailable() {
		if e.router.LocalAvailable() {
			result.Warnings = append(result.Warnings,
				"Cloud API unavailable, sir. Using local LLM for synthesis — results may be less thorough.")
			synthesis, err := e.summarize(ctx, query, result.Results[:maxFetch])
			if err == nil {
				result.Synthesis = synthesis
			}
		} else {
			result.Warnings = append(result.Warnings,
				"No LLM providers available, sir. Presenting raw results.")
		}
	} else {
		synthesis, err := e.synthesize(ctx, query, result.Results[:maxFetch])
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Synthesis encountered difficulties, sir: %v", err))
		} else {
			result.Synthesis = synthesis
		}
	}

	// Persist to store if resolution warrants it
	if res.ShouldPersist() && e.store != nil && topicID != nil {
		persisted := e.persist(ctx, result, topicID)
		result.Persisted = persisted
	}

	return result, nil
}

func (e *Engine) fetchContent(ctx context.Context, results []source.Result) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // Limit concurrency

	for i := range results {
		if results[i].Content != "" || results[i].URL == "" {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			content, err := e.fetcher.Fetch(ctx, results[idx].URL)
			if err == nil {
				results[idx].Content = content
			}
		}(i)
	}
	wg.Wait()
}

func (e *Engine) summarize(ctx context.Context, query string, results []source.Result) (string, error) {
	var sb strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sb, "## Source %d: %s\n%s\n\n", i+1, r.Title, contentOrSnippet(r))
	}

	return e.router.Complete(ctx, llm.TaskSummarize, llm.Request{
		System: `You are a research assistant. Summarize the following sources to answer the user's query.
Be concise, accurate, and cite sources by number. Use markdown formatting.`,
		Prompt: fmt.Sprintf("Query: %s\n\nSources:\n%s", query, sb.String()),
		MaxTok: 2000,
	})
}

func (e *Engine) synthesize(ctx context.Context, query string, results []source.Result) (string, error) {
	var sb strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sb, "## Source %d: %s\nURL: %s\n%s\n\n", i+1, r.Title, r.URL, contentOrSnippet(r))
	}

	return e.router.Complete(ctx, llm.TaskSynthesize, llm.Request{
		System: `You are a thorough research synthesizer. Analyze all provided sources to create a comprehensive answer to the user's query.
Structure your response with clear sections, cite sources, note any contradictions or gaps, and highlight key takeaways.
Use markdown formatting.`,
		Prompt: fmt.Sprintf("Research query: %s\n\nSources:\n%s", query, sb.String()),
		MaxTok: 4096,
	})
}

func (e *Engine) persist(ctx context.Context, result *ResearchResult, topicID *int64) int {
	count := 0

	// Persist synthesis
	if result.Synthesis != "" {
		_, err := e.store.AddKnowledge(store.KnowledgeInput{
			TopicID: topicID,
			Type:    store.KnowledgeSynthesis,
			Title:   fmt.Sprintf("Research: %s", result.Query),
			Content: result.Synthesis,
			Score:   1.0,
		})
		if err == nil {
			count++
		}
	}

	// Persist top sources
	for _, r := range result.Results {
		if r.URL == "" {
			continue
		}
		url := r.URL
		_, err := e.store.AddKnowledge(store.KnowledgeInput{
			TopicID: topicID,
			Type:    store.KnowledgeSource,
			Title:   r.Title,
			Content: contentOrSnippet(r),
			URL:     &url,
			Score:   r.Score,
		})
		if err == nil {
			count++
		}
	}

	return count
}

func contentOrSnippet(r source.Result) string {
	if r.Content != "" {
		return r.Content
	}
	return r.Snippet
}
