package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type SearXNG struct {
	endpoint string
	client   *http.Client
}

func NewSearXNG(endpoint string) *SearXNG {
	return &SearXNG{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SearXNG) Name() string { return "searxng" }

type searxngResponse struct {
	Results []struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	} `json:"results"`
}

func (s *SearXNG) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if s.endpoint == "" {
		return nil, fmt.Errorf("SearXNG endpoint not configured")
	}
	if limit <= 0 {
		limit = 10
	}

	u := fmt.Sprintf("%s/search?q=%s&format=json&pageno=1",
		s.endpoint, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searxng request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("searxng returned status %d: %s", resp.StatusCode, string(body))
	}

	var sResp searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&sResp); err != nil {
		return nil, fmt.Errorf("decoding searxng response: %w", err)
	}

	results := make([]Result, 0, min(limit, len(sResp.Results)))
	for i, r := range sResp.Results {
		if i >= limit {
			break
		}
		score := r.Score
		if score == 0 {
			score = 1.0 - float64(i)*0.05
		}
		results = append(results, Result{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Score:   score,
		})
	}

	return results, nil
}
