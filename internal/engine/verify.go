package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexfsong/jeeves/internal/config"
	"github.com/alexfsong/jeeves/internal/llm"
	"github.com/alexfsong/jeeves/internal/source"
)

const verifySystemPrompt = `You are a meticulous verification analyst assisting a research assistant named Jeeves.

You will be given a research query, a synthesis drafted from an initial set of sources, and the URLs of those sources. Your task is to use the web_search and web_fetch tools to:

1. Verify 2–4 load-bearing factual claims in the synthesis.
2. Fill in gaps the synthesis itself flagged as missing or uncertain.
3. Surface contradictions between sources, or between the synthesis and what you find.

Do NOT rewrite or reproduce the synthesis. Produce a concise addendum, formatted as markdown, under exactly these headings:

## Verification
## Corrections
## New findings

If a section has nothing to report, write "None." under that heading. Cite URLs inline where relevant. Be terse; this is an addendum, not a report.

You may skip the URLs already provided — the user has already read them. Prefer fresh sources.`

type verifyOutput struct {
	Notes      string
	NewSources []source.Result
	ToolCalls  int
}

// verify runs the post-synthesis verification pass using Anthropic's native
// web_search + web_fetch tools. Called only when cfg.Enabled is true and a
// cloud provider is available.
func (e *Engine) verify(
	ctx context.Context,
	query, synthesis string,
	existing []source.Result,
	cfg config.VerifyConfig,
) (*verifyOutput, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if !e.router.CloudAvailable() {
		return nil, fmt.Errorf("cloud LLM unavailable")
	}

	// Bound the whole verification pass — server tools can stall.
	vctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	var existingURLs strings.Builder
	for _, r := range existing {
		if r.URL == "" {
			continue
		}
		fmt.Fprintf(&existingURLs, "- %s\n", r.URL)
	}

	prompt := fmt.Sprintf(
		"Research query:\n%s\n\n---\n\nSynthesis to verify:\n%s\n\n---\n\nURLs already consulted (skip these):\n%s",
		query, synthesis, strings.TrimSpace(existingURLs.String()),
	)

	resp, err := e.router.VerifyWithWeb(vctx, llm.VerifyRequest{
		System:         verifySystemPrompt,
		Prompt:         prompt,
		MaxTok:         cfg.MaxTokens,
		MaxSearchUses:  cfg.MaxSearchUses,
		MaxFetchUses:   cfg.MaxFetchUses,
		AllowedDomains: cfg.AllowedDomains,
		BlockedDomains: cfg.BlockedDomains,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	out := &verifyOutput{
		Notes:     resp.Text,
		ToolCalls: resp.ToolCalls,
	}

	// Dedupe against existing URLs the original pipeline already has.
	existingSet := make(map[string]bool, len(existing))
	for _, r := range existing {
		if r.URL != "" {
			existingSet[r.URL] = true
		}
	}

	cap := cfg.MaxNewSources
	if cap <= 0 {
		cap = 5
	}

	for _, s := range resp.Sources {
		if s.URL == "" || existingSet[s.URL] {
			continue
		}
		existingSet[s.URL] = true
		out.NewSources = append(out.NewSources, source.Result{
			Title:    firstNonEmpty(s.Title, s.URL),
			URL:      s.URL,
			Snippet:  "",
			Content:  s.Content,
			Score:    0.5,
			Verified: true,
		})
		if len(out.NewSources) >= cap {
			break
		}
	}

	return out, nil
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
