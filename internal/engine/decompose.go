package engine

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/alexfsong/jeeves/internal/llm"
	"github.com/alexfsong/jeeves/internal/resolution"
)

// Decompose breaks a query into sub-queries.
// Glance/brief: heuristic decomposition. Detailed+: LLM decomposition.
func Decompose(ctx context.Context, query string, res resolution.Level, router *llm.Router) []string {
	if res.UseLLMDecomposition() && (router.LocalAvailable() || router.CloudAvailable()) {
		subs, err := llmDecompose(ctx, query, router)
		if err == nil && len(subs) > 0 {
			return subs
		}
		// Fall through to heuristic on failure
	}
	return heuristicDecompose(query)
}

func heuristicDecompose(query string) []string {
	// Split on conjunctions and question marks
	parts := strings.FieldsFunc(query, func(r rune) bool {
		return r == '?' || r == ';'
	})

	var subs []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split on "and" / "or" / "vs" at word boundaries
		for _, conj := range []string{" and ", " or ", " vs ", " versus "} {
			if strings.Contains(strings.ToLower(part), conj) {
				pieces := strings.SplitN(strings.ToLower(part), conj, 2)
				for _, p := range pieces {
					p = strings.TrimSpace(p)
					if p != "" {
						subs = append(subs, p)
					}
				}
				part = "" // Already split
				break
			}
		}

		if part != "" {
			subs = append(subs, part)
		}
	}

	if len(subs) == 0 {
		return []string{query}
	}
	return subs
}

func llmDecompose(ctx context.Context, query string, router *llm.Router) ([]string, error) {
	resp, err := router.Complete(ctx, llm.TaskDecompose, llm.Request{
		System: `You are a research query decomposer. Break the user's query into 1-5 focused sub-queries for web search.
Return a JSON array of strings. Only output the JSON array, nothing else.
If the query is already focused, return it as a single-element array.`,
		Prompt: query,
		MaxTok: 500,
	})
	if err != nil {
		return nil, err
	}

	// Parse JSON array
	resp = strings.TrimSpace(resp)
	var subs []string
	if err := json.Unmarshal([]byte(resp), &subs); err != nil {
		// Try to extract from markdown code block
		if idx := strings.Index(resp, "["); idx >= 0 {
			if end := strings.LastIndex(resp, "]"); end > idx {
				if err := json.Unmarshal([]byte(resp[idx:end+1]), &subs); err != nil {
					return nil, err
				}
			}
		}
		if len(subs) == 0 {
			return nil, err
		}
	}

	return subs, nil
}
