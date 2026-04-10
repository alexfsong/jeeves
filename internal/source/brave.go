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

type Brave struct {
	apiKey string
	client *http.Client
}

func NewBrave(apiKey string) *Brave {
	return &Brave{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *Brave) Name() string { return "brave" }

type braveResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

func (b *Brave) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if b.apiKey == "" {
		return nil, fmt.Errorf("brave API key not configured")
	}
	if limit <= 0 {
		limit = 10
	}

	u := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Subscription-Token", b.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("brave API returned status %d: %s", resp.StatusCode, string(body))
	}

	var bResp braveResponse
	if err := json.NewDecoder(resp.Body).Decode(&bResp); err != nil {
		return nil, fmt.Errorf("decoding brave response: %w", err)
	}

	results := make([]Result, 0, len(bResp.Web.Results))
	for i, r := range bResp.Web.Results {
		results = append(results, Result{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
			Score:   1.0 - float64(i)*0.05, // Position-based scoring
		})
	}

	return results, nil
}
