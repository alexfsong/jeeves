package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/alexfsong/jeeves/internal/config"
	"github.com/alexfsong/jeeves/internal/llm"
	"github.com/alexfsong/jeeves/internal/resolution"
	"github.com/alexfsong/jeeves/internal/source"
	"github.com/alexfsong/jeeves/internal/store"
)

type Engine struct {
	search    source.Source
	wiki      *source.WikiSource
	fetcher   *source.Fetcher
	router    *llm.Router
	store     *store.Store
	verifyCfg config.VerifyConfig
}

// ProgressEvent is emitted during ResearchStream to signal pipeline stage
// transitions to the caller (typically the API handler forwarding SSE).
type ProgressEvent struct {
	Stage      string `json:"stage"`
	Detail     string `json:"detail,omitempty"`
	ToolCalls  int    `json:"tool_calls,omitempty"`
	NewSources int    `json:"new_sources,omitempty"`
}

// VerificationInfo summarizes what the verification pass produced.
type VerificationInfo struct {
	ToolCalls  int  `json:"tool_calls"`
	NewSources int  `json:"new_sources"`
	Skipped    bool `json:"skipped,omitempty"`
}

type ResearchResult struct {
	Query        string            `json:"query"`
	Resolution   string            `json:"resolution"`
	SubQueries   []string          `json:"sub_queries,omitempty"`
	Results      []source.Result   `json:"results"`
	Synthesis    string            `json:"synthesis,omitempty"`
	Warnings     []string          `json:"warnings,omitempty"`
	Persisted    int               `json:"persisted,omitempty"`
	Verification *VerificationInfo `json:"verification,omitempty"`
}

func New(search source.Source, wiki *source.WikiSource, fetcher *source.Fetcher, router *llm.Router, st *store.Store, verifyCfg config.VerifyConfig) *Engine {
	return &Engine{
		search:    search,
		wiki:      wiki,
		fetcher:   fetcher,
		router:    router,
		store:     st,
		verifyCfg: verifyCfg,
	}
}

func (e *Engine) Research(ctx context.Context, query string, res resolution.Level, topicID *int64) (*ResearchResult, error) {
	return e.ResearchStream(ctx, query, res, topicID, nil)
}

// ResearchStream runs the research pipeline, optionally emitting progress
// events on the provided channel. If progress is nil, events are dropped.
// The caller is responsible for reading from the channel; events are sent
// non-blocking so a slow consumer can't stall the pipeline.
func (e *Engine) ResearchStream(ctx context.Context, query string, res resolution.Level, topicID *int64, progress chan<- ProgressEvent) (*ResearchResult, error) {
	emit := func(ev ProgressEvent) {
		if progress == nil {
			return
		}
		select {
		case progress <- ev:
		default:
		}
	}

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
	emit(ProgressEvent{Stage: "decompose"})
	subQueries := Decompose(ctx, query, res, e.router)
	result.SubQueries = subQueries

	// Search
	emit(ProgressEvent{Stage: "search"})
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
		emit(ProgressEvent{Stage: "fetch"})
		e.fetchContent(ctx, result.Results[:maxFetch])
	}

	// Brief: local LLM summarize
	if res == resolution.Brief {
		if !e.router.LocalAvailable() {
			result.Warnings = append(result.Warnings,
				"Local LLM unavailable, sir. Falling back to glance resolution.")
			return result, nil
		}
		emit(ProgressEvent{Stage: "synthesize", Detail: "local summarize"})
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
			emit(ProgressEvent{Stage: "synthesize", Detail: "local fallback"})
			synthesis, err := e.summarize(ctx, query, result.Results[:maxFetch])
			if err == nil {
				result.Synthesis = synthesis
			}
		} else {
			result.Warnings = append(result.Warnings,
				"No LLM providers available, sir. Presenting raw results.")
		}
	} else {
		emit(ProgressEvent{Stage: "synthesize"})
		synthesis, err := e.synthesize(ctx, query, result.Results[:maxFetch])
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Synthesis encountered difficulties, sir: %v", err))
		} else {
			result.Synthesis = synthesis
		}
	}

	// Optional verification pass — opt-in, cloud-only, strictly budgeted.
	if res >= resolution.Detailed && e.verifyCfg.Enabled && result.Synthesis != "" && e.router.CloudAvailable() {
		emit(ProgressEvent{Stage: "verify", Detail: "invoking web tools"})
		vo, err := e.verify(ctx, query, result.Synthesis, result.Results[:maxFetch], e.verifyCfg)
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Verification pass unavailable, sir: %v", err))
			result.Verification = &VerificationInfo{Skipped: true}
		} else if vo != nil {
			if trimmed := strings.TrimSpace(vo.Notes); trimmed != "" {
				result.Synthesis += "\n\n---\n\n## Verification addendum\n\n" + trimmed
			}
			if len(vo.NewSources) > 0 {
				// Respect user trust signals on verified sources, but do not
				// globally re-rank — verified sources append at the end so
				// UI ordering stays stable.
				if e.store != nil {
					if trusted, terr := e.store.ListTrustedSources(topicID); terr == nil && len(trusted) > 0 {
						ApplyTrustBoost(vo.NewSources, trusted)
					}
				}
				result.Results = append(result.Results, vo.NewSources...)
			}
			result.Verification = &VerificationInfo{
				ToolCalls:  vo.ToolCalls,
				NewSources: len(vo.NewSources),
			}
			emit(ProgressEvent{
				Stage:      "verify",
				Detail:     "complete",
				ToolCalls:  vo.ToolCalls,
				NewSources: len(vo.NewSources),
			})
		}
	}

	// Persist to store if resolution warrants it
	if res.ShouldPersist() && e.store != nil && topicID != nil {
		emit(ProgressEvent{Stage: "persist"})
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
		ktype := store.KnowledgeSource
		if r.Verified {
			ktype = store.KnowledgeVerifiedSource
		}
		_, err := e.store.AddKnowledge(store.KnowledgeInput{
			TopicID: topicID,
			Type:    ktype,
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

// PriorKnowledge checks the local knowledge base for existing info on a query.
func (e *Engine) PriorKnowledge(ctx context.Context, query string) ([]store.Knowledge, error) {
	if e.store == nil {
		return nil, nil
	}
	return e.store.SearchKnowledge(query, 10)
}

// GapAnalysis examines what's been researched for a topic and identifies gaps.
func (e *Engine) GapAnalysis(ctx context.Context, topicID int64) (string, error) {
	knowledge, err := e.store.ListKnowledgeByTopic(topicID)
	if err != nil {
		return "", err
	}
	if len(knowledge) == 0 {
		return "No knowledge entries for this topic yet. Start researching to build your knowledge base.", nil
	}

	// Build a summary of what exists
	var sb strings.Builder
	for _, k := range knowledge {
		fmt.Fprintf(&sb, "- [%s] %s\n", k.Type, k.Title)
	}

	resp, err := e.router.Complete(ctx, llm.TaskGapAnalysis, llm.Request{
		System: `You are a research advisor. Given a list of knowledge entries for a research topic, identify:
1. What areas have been well covered
2. What important gaps or blind spots exist
3. Specific follow-up research questions to fill those gaps
4. Any potential contradictions worth investigating

Be concise and actionable. Use markdown formatting.`,
		Prompt: fmt.Sprintf("Knowledge entries for this topic:\n%s\n\nProvide a gap analysis.", sb.String()),
		MaxTok: 1500,
	})
	if err != nil {
		return "", err
	}

	return resp, nil
}

// GenerateFollowUps produces follow-up research questions from a synthesis.
func (e *Engine) GenerateFollowUps(ctx context.Context, query, synthesis string) ([]string, error) {
	prompt := fmt.Sprintf(`Based on this research query and synthesis, suggest 3-5 follow-up questions that would deepen the research. These should be specific, actionable questions that explore different angles or gaps in the current findings.

Query: %s

Synthesis:
%s

Return ONLY a JSON array of strings, nothing else. Example: ["question 1", "question 2", "question 3"]`, query, synthesis)

	resp, err := e.router.Complete(ctx, llm.TaskSummarize, llm.Request{
		System: "You generate follow-up research questions. Return only a JSON array of question strings.",
		Prompt: prompt,
		MaxTok: 500,
	})
	if err != nil {
		return nil, err
	}

	// Parse the JSON array from the response
	resp = strings.TrimSpace(resp)
	// Handle case where LLM wraps in markdown code block
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var questions []string
	if err := json.Unmarshal([]byte(resp), &questions); err != nil {
		// Fallback: split by newlines if JSON parsing fails
		lines := strings.Split(resp, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimLeft(line, "0123456789.-) ")
			if line != "" && len(line) > 10 {
				questions = append(questions, line)
			}
		}
	}

	return questions, nil
}
