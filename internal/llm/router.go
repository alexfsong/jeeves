package llm

import (
	"context"
	"fmt"

	"github.com/alexfsong/jeeves/internal/config"
)

type Task string

const (
	TaskDecompose   Task = "decompose"
	TaskSummarize   Task = "summarize"
	TaskRerank      Task = "rerank"
	TaskSynthesize  Task = "synthesize"
	TaskGapAnalysis Task = "gap_analysis"
	TaskWikiLint    Task = "wiki_lint"
)

type Provider interface {
	Name() string
	Available() bool
	Complete(ctx context.Context, req Request) (string, error)
}

type Request struct {
	System string
	Prompt string
	MaxTok int
}

type Router struct {
	local Provider
	cloud Provider
	force string
}

func NewRouter(cfg config.Config) *Router {
	ollama := NewOllama(cfg.LLM.LocalEndpoint, cfg.LLM.LocalModel)
	// Auto-resolve model: if configured model isn't available, pick first installed one
	ollama.ResolveModel()

	r := &Router{
		local: ollama,
		cloud: NewAnthropic(cfg.LLM.CloudAPIKey, cfg.LLM.CloudModel),
		force: cfg.LLM.ForceProvider,
	}
	return r
}

// LocalOllama returns the underlying Ollama provider (for model listing, etc).
func (r *Router) LocalOllama() *Ollama {
	if o, ok := r.local.(*Ollama); ok {
		return o
	}
	return nil
}

// taskProvider returns the appropriate provider for a task.
var taskProviderMap = map[Task]string{
	TaskDecompose:   "local",
	TaskSummarize:   "local",
	TaskRerank:      "local",
	TaskSynthesize:  "cloud",
	TaskGapAnalysis: "local", // basic; detailed uses cloud
	TaskWikiLint:    "cloud",
}

func (r *Router) Complete(ctx context.Context, task Task, req Request) (string, error) {
	provider := r.selectProvider(task)
	if provider == nil {
		return "", fmt.Errorf("no LLM provider available for task %q", task)
	}
	return provider.Complete(ctx, req)
}

func (r *Router) selectProvider(task Task) Provider {
	if r.force == "local" && r.local.Available() {
		return r.local
	}
	if r.force == "cloud" && r.cloud.Available() {
		return r.cloud
	}

	preferred := taskProviderMap[task]
	if preferred == "cloud" {
		if r.cloud.Available() {
			return r.cloud
		}
		if r.local.Available() {
			return r.local
		}
		return nil
	}

	// Prefer local
	if r.local.Available() {
		return r.local
	}
	if r.cloud.Available() {
		return r.cloud
	}
	return nil
}

func (r *Router) LocalAvailable() bool {
	return r.local.Available()
}

func (r *Router) CloudAvailable() bool {
	return r.cloud.Available()
}

// VerifyWithWeb routes a verification request to the cloud provider's
// server-side web_search + web_fetch tools. Cloud-only by design — the
// local provider has no equivalent capability.
func (r *Router) VerifyWithWeb(ctx context.Context, req VerifyRequest) (*VerifyResponse, error) {
	a, ok := r.cloud.(*Anthropic)
	if !ok || a == nil {
		return nil, fmt.Errorf("verification requires an Anthropic cloud provider")
	}
	if !a.Available() {
		return nil, fmt.Errorf("anthropic API key not configured")
	}
	return a.CompleteWithWebTools(ctx, req)
}
