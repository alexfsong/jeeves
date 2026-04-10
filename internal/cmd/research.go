package cmd

import (
	"fmt"
	"strings"

	"github.com/alexfsong/jeeves/internal/config"
	"github.com/alexfsong/jeeves/internal/engine"
	"github.com/alexfsong/jeeves/internal/llm"
	"github.com/alexfsong/jeeves/internal/render"
	"github.com/alexfsong/jeeves/internal/resolution"
	"github.com/alexfsong/jeeves/internal/source"
	"github.com/alexfsong/jeeves/internal/store"
	"github.com/spf13/cobra"
)

var (
	resolutionFlag string
	topicFlag      string
)

var researchCmd = &cobra.Command{
	Use:   "research <query>",
	Short: "Research a topic",
	Long:  "Research a topic at the specified resolution level. Jeeves will search, fetch, and optionally synthesize results.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		r := render.New(jsonOutput)

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Resolve resolution
		resStr := resolutionFlag
		if resStr == "" {
			resStr = cfg.Defaults.Resolution
		}
		res, err := resolution.Parse(resStr)
		if err != nil {
			return err
		}

		// Open store
		st, err := store.Open()
		if err != nil {
			return fmt.Errorf("opening store: %w", err)
		}
		defer st.Close()

		// Resolve topic
		var topicID *int64
		if topicFlag != "" {
			topic, err := st.GetTopicBySlug(topicFlag)
			if err != nil {
				return fmt.Errorf("topic: %w", err)
			}
			topicID = &topic.ID
		}

		// Build search source
		search := buildSearchSource(cfg)

		// Build engine
		router := llm.NewRouter(cfg)
		wiki := source.NewWikiSource(st)
		fetcher := source.NewFetcher()
		eng := engine.New(search, wiki, fetcher, router, st)

		// Research
		ctx := cmd.Context()
		result, err := eng.Research(ctx, query, res, topicID)
		if err != nil {
			return err
		}

		// Log session
		st.LogSession(topicID, query, res.String(), len(result.Results))

		// Render
		if r.IsJSON() {
			r.JSON(result)
			return nil
		}

		renderResearchResult(r, result)
		return nil
	},
}

func buildSearchSource(cfg config.Config) source.Source {
	switch cfg.Search.Provider {
	case "searxng":
		return source.NewSearXNG(cfg.Search.SearXNGEndpoint)
	case "tavily":
		return source.NewTavily(cfg.Search.TavilyAPIKey)
	default:
		return source.NewBrave(cfg.Search.BraveAPIKey)
	}
}

func renderResearchResult(r *render.Renderer, result *engine.ResearchResult) {
	r.Title(fmt.Sprintf("Research: %s", result.Query))
	r.Meta(fmt.Sprintf("Resolution: %s", result.Resolution))

	if len(result.SubQueries) > 1 {
		r.Meta(fmt.Sprintf("Sub-queries: %s", strings.Join(result.SubQueries, ", ")))
	}

	for _, w := range result.Warnings {
		r.Warn(w)
	}

	r.Divider()

	if result.Synthesis != "" {
		r.Header("Synthesis")
		r.Markdown(result.Synthesis)
		r.Divider()
	}

	r.Header(fmt.Sprintf("Sources (%d)", len(result.Results)))
	for i, res := range result.Results {
		if i >= 10 {
			r.Meta(fmt.Sprintf("  ... and %d more", len(result.Results)-10))
			break
		}
		r.Body(fmt.Sprintf("  %d. %s", i+1, res.Title))
		if res.URL != "" {
			r.URL(fmt.Sprintf("     %s", res.URL))
		}
		if res.Snippet != "" {
			snippet := res.Snippet
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			r.Body(fmt.Sprintf("     %s", snippet))
		}
		r.Score("relevance", res.Score)
	}

	if result.Persisted > 0 {
		r.Success(fmt.Sprintf("\nVery good, sir. %d items persisted to your knowledge base.", result.Persisted))
	}
}

func init() {
	researchCmd.Flags().StringVarP(&resolutionFlag, "resolution", "r", "", "Resolution level: glance, brief, detailed, full")
	researchCmd.Flags().StringVarP(&topicFlag, "topic", "t", "", "Associate results with a topic (by slug)")
	rootCmd.AddCommand(researchCmd)
}
