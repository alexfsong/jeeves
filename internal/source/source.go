package source

import "context"

type Result struct {
	Title    string  `json:"title"`
	URL      string  `json:"url"`
	Snippet  string  `json:"snippet"`
	Content  string  `json:"content,omitempty"` // Populated after fetch
	Score    float64 `json:"score"`
	Verified bool    `json:"verified,omitempty"` // Surfaced by the post-synthesis verification pass
}

type Source interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]Result, error)
}
