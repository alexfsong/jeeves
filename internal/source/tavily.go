package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Tavily struct {
	apiKey string
	client *http.Client
}

func NewTavily(apiKey string) *Tavily {
	return &Tavily{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *Tavily) Name() string { return "tavily" }

type tavilyRequest struct {
	APIKey           string `json:"api_key"`
	Query            string `json:"query"`
	MaxResults       int    `json:"max_results"`
	IncludeRawContent bool  `json:"include_raw_content"`
}

type tavilyResponse struct {
	Results []struct {
		Title      string  `json:"title"`
		URL        string  `json:"url"`
		Content    string  `json:"content"`
		RawContent string  `json:"raw_content"`
		Score      float64 `json:"score"`
	} `json:"results"`
}

func (t *Tavily) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if t.apiKey == "" {
		return nil, fmt.Errorf("tavily API key not configured")
	}
	if limit <= 0 {
		limit = 10
	}

	body := tavilyRequest{
		APIKey:     t.apiKey,
		Query:      query,
		MaxResults: limit,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tavily returned status %d: %s", resp.StatusCode, string(b))
	}

	var tResp tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&tResp); err != nil {
		return nil, fmt.Errorf("decoding tavily response: %w", err)
	}

	results := make([]Result, 0, len(tResp.Results))
	for _, r := range tResp.Results {
		results = append(results, Result{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Content: r.RawContent,
			Score:   r.Score,
		})
	}

	return results, nil
}
