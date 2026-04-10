package llm

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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
