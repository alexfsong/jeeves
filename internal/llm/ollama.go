package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Ollama struct {
	endpoint string
	model    string
	client   *http.Client
}

func NewOllama(endpoint, model string) *Ollama {
	return &Ollama{
		endpoint: endpoint,
		model:    model,
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *Ollama) Name() string { return "ollama" }

func (o *Ollama) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", o.endpoint+"/api/tags", nil)
	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system,omitempty"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func (o *Ollama) Complete(ctx context.Context, req Request) (string, error) {
	body := ollamaRequest{
		Model:  o.model,
		Prompt: req.Prompt,
		System: req.System,
		Stream: false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.endpoint+"/api/generate", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(b))
	}

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding ollama response: %w", err)
	}

	return result.Response, nil
}
