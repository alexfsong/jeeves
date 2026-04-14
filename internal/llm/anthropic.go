package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

type Anthropic struct {
	apiKey string
	model  string
	client *anthropic.Client
}

func NewAnthropic(apiKey, model string) *Anthropic {
	a := &Anthropic{
		apiKey: apiKey,
		model:  model,
	}
	if apiKey != "" {
		client := anthropic.NewClient(option.WithAPIKey(apiKey))
		a.client = &client
	}
	return a
}

func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) Available() bool {
	return a.apiKey != "" && a.client != nil
}

func (a *Anthropic) Complete(ctx context.Context, req Request) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("anthropic API key not configured")
	}

	maxTok := int64(req.MaxTok)
	if maxTok <= 0 {
		maxTok = 4096
	}

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: maxTok,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt)),
		},
	}

	if req.System != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.System},
		}
	}

	msg, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("anthropic API error: %w", err)
	}

	for _, block := range msg.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("no text content in anthropic response")
}

// VerifyRequest drives CompleteWithWebTools, Anthropic's server-side
// web_search + web_fetch tool invocation. Budgets are hard caps relayed
// directly to the Anthropic API.
type VerifyRequest struct {
	System         string
	Prompt         string
	MaxTok         int
	MaxSearchUses  int
	MaxFetchUses   int
	AllowedDomains []string
	BlockedDomains []string
}

// VerifySource is a URL surfaced by a server-side tool call — either a
// web_search result hit or a web_fetch'd document.
type VerifySource struct {
	URL         string
	Title       string
	Content     string // populated for web_fetch text results
	Kind        string // "search" or "fetch"
	RetrievedAt string
}

// VerifyResponse is the parsed output of a CompleteWithWebTools call.
type VerifyResponse struct {
	Text      string
	Sources   []VerifySource
	ToolCalls int
}

// CompleteWithWebTools runs a single Claude Messages call with the server-side
// web_search_20250305 + web_fetch_20250910 tools attached, then parses the
// resulting content blocks into a verification response. No client-side tool
// loop is needed — Anthropic executes the tools server-side and returns the
// trace inline.
func (a *Anthropic) CompleteWithWebTools(ctx context.Context, req VerifyRequest) (*VerifyResponse, error) {
	if !a.Available() {
		return nil, fmt.Errorf("anthropic API key not configured")
	}

	maxTok := int64(req.MaxTok)
	if maxTok <= 0 {
		maxTok = 2048
	}

	searchTool := &anthropic.WebSearchTool20250305Param{}
	if req.MaxSearchUses > 0 {
		searchTool.MaxUses = param.NewOpt(int64(req.MaxSearchUses))
	}
	if len(req.AllowedDomains) > 0 {
		searchTool.AllowedDomains = req.AllowedDomains
	} else if len(req.BlockedDomains) > 0 {
		searchTool.BlockedDomains = req.BlockedDomains
	}

	fetchTool := &anthropic.WebFetchTool20250910Param{}
	if req.MaxFetchUses > 0 {
		fetchTool.MaxUses = param.NewOpt(int64(req.MaxFetchUses))
	}
	if len(req.AllowedDomains) > 0 {
		fetchTool.AllowedDomains = req.AllowedDomains
	} else if len(req.BlockedDomains) > 0 {
		fetchTool.BlockedDomains = req.BlockedDomains
	}

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: maxTok,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt)),
		},
		Tools: []anthropic.ToolUnionParam{
			{OfWebSearchTool20250305: searchTool},
			{OfWebFetchTool20250910: fetchTool},
		},
	}

	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}

	msg, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic web-tools API error: %w", err)
	}

	resp := &VerifyResponse{}
	seen := map[string]int{} // URL → index in resp.Sources (for fetch-upgrades-search)

	var textParts []string
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
			}
		case "server_tool_use":
			resp.ToolCalls++
		case "web_search_tool_result":
			sr := block.AsWebSearchToolResult()
			for _, item := range sr.Content.OfWebSearchResultBlockArray {
				if item.URL == "" {
					continue
				}
				if _, ok := seen[item.URL]; ok {
					continue
				}
				resp.Sources = append(resp.Sources, VerifySource{
					URL:   item.URL,
					Title: item.Title,
					Kind:  "search",
				})
				seen[item.URL] = len(resp.Sources) - 1
			}
		case "web_fetch_tool_result":
			fr := block.AsWebFetchToolResult()
			// Try to pull out the web_fetch result block (ignore error blocks).
			doc := fr.Content.AsResponseWebFetchResultBlock()
			url := doc.URL
			if url == "" {
				continue
			}
			text := ""
			if src := doc.Content.Source; src.Type == "text" {
				text = src.AsText().Data
			}
			if idx, ok := seen[url]; ok {
				// upgrade the search hit with fetched content
				resp.Sources[idx].Content = text
				resp.Sources[idx].Kind = "fetch"
				if resp.Sources[idx].Title == "" {
					resp.Sources[idx].Title = doc.Content.Title
				}
				if doc.RetrievedAt != "" {
					resp.Sources[idx].RetrievedAt = doc.RetrievedAt
				}
				continue
			}
			resp.Sources = append(resp.Sources, VerifySource{
				URL:         url,
				Title:       doc.Content.Title,
				Content:     text,
				Kind:        "fetch",
				RetrievedAt: doc.RetrievedAt,
			})
			seen[url] = len(resp.Sources) - 1
		}
	}

	resp.Text = strings.TrimSpace(strings.Join(textParts, "\n\n"))
	return resp, nil
}
