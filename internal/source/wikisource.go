package source

import (
	"context"

	"github.com/alexfsong/jeeves/internal/store"
)

// WikiSource queries the local knowledge store as a source.
type WikiSource struct {
	store *store.Store
}

func NewWikiSource(s *store.Store) *WikiSource {
	return &WikiSource{store: s}
}

func (w *WikiSource) Name() string { return "wiki" }

func (w *WikiSource) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	entries, err := w.store.SearchKnowledge(query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(entries))
	for _, e := range entries {
		url := ""
		if e.URL != nil {
			url = *e.URL
		}
		results = append(results, Result{
			Title:   e.Title,
			URL:     url,
			Snippet: truncate(e.Content, 300),
			Content: e.Content,
			Score:   e.Score,
		})
	}
	return results, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
