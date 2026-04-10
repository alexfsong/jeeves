package cmd

import (
	"fmt"
	"strings"

	"github.com/alexfsong/jeeves/internal/config"
	"github.com/alexfsong/jeeves/internal/llm"
	"github.com/alexfsong/jeeves/internal/render"
	"github.com/alexfsong/jeeves/internal/store"
	"github.com/spf13/cobra"
)

var wikiCmd = &cobra.Command{
	Use:   "wiki",
	Short: "Query the local knowledge base",
}

var wikiSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the knowledge base",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		r := render.New(jsonOutput)

		st, err := store.Open()
		if err != nil {
			return err
		}
		defer st.Close()

		results, err := st.SearchKnowledge(query, 20)
		if err != nil {
			return fmt.Errorf("searching: %w", err)
		}

		if r.IsJSON() {
			r.JSON(results)
			return nil
		}

		if len(results) == 0 {
			r.Body("No results found, sir. The knowledge base may need enriching.")
			return nil
		}

		r.Title(fmt.Sprintf("Wiki Search: %s", query))
		r.Divider()
		for i, k := range results {
			r.Header(fmt.Sprintf("  %d. %s", i+1, k.Title))
			r.Meta(fmt.Sprintf("     type: %s | score: %.2f | %s", k.Type, k.Score, k.CreatedAt.Format("2006-01-02")))
			if k.URL != nil {
				r.URL(fmt.Sprintf("     %s", *k.URL))
			}
			snippet := k.Content
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			r.Body(fmt.Sprintf("     %s", snippet))
		}
		return nil
	},
}

var wikiShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a knowledge entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r := render.New(jsonOutput)

		var id int64
		fmt.Sscanf(args[0], "%d", &id)

		st, err := store.Open()
		if err != nil {
			return err
		}
		defer st.Close()

		k, err := st.GetKnowledge(id)
		if err != nil {
			return fmt.Errorf("entry not found: %w", err)
		}

		if r.IsJSON() {
			r.JSON(k)
			return nil
		}

		r.Title(k.Title)
		r.Meta(fmt.Sprintf("Type: %s | Score: %.2f | Created: %s", k.Type, k.Score, k.CreatedAt.Format("2006-01-02 15:04")))
		if k.URL != nil {
			r.URL(*k.URL)
		}
		r.Divider()
		r.Markdown(k.Content)
		return nil
	},
}

var wikiLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all knowledge entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := render.New(jsonOutput)

		st, err := store.Open()
		if err != nil {
			return err
		}
		defer st.Close()

		entries, err := st.ListAllKnowledge(50)
		if err != nil {
			return err
		}

		if r.IsJSON() {
			r.JSON(entries)
			return nil
		}

		if len(entries) == 0 {
			r.Body("The knowledge base is empty, sir. Try: jeeves research \"topic\" -r detailed")
			return nil
		}

		r.Title("Knowledge Base")
		r.Divider()
		for _, k := range entries {
			r.Body(fmt.Sprintf("  [%d] %s", k.ID, k.Title))
			r.Meta(fmt.Sprintf("       type: %s | score: %.2f | %s", k.Type, k.Score, k.CreatedAt.Format("2006-01-02")))
		}
		return nil
	},
}

var wikiLintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Check knowledge base for quality issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := render.New(jsonOutput)

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		st, err := store.Open()
		if err != nil {
			return err
		}
		defer st.Close()

		entries, err := st.ListAllKnowledge(0)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			r.Body("Nothing to lint, sir.")
			return nil
		}

		var sb strings.Builder
		for _, e := range entries {
			fmt.Fprintf(&sb, "- [%d] [%s] %s (score: %.2f)\n", e.ID, e.Type, e.Title, e.Score)
		}

		router := llm.NewRouter(cfg)
		analysis, err := router.Complete(cmd.Context(), llm.TaskWikiLint, llm.Request{
			System: `You are a knowledge base quality checker. Review the entries and identify:
1. Duplicate or near-duplicate entries
2. Entries with low scores that might be noise
3. Missing types (e.g., many sources but no syntheses)
4. Suggested cleanup actions

Be concise. Use markdown.`,
			Prompt: fmt.Sprintf("Knowledge base (%d entries):\n%s", len(entries), sb.String()),
			MaxTok: 2000,
		})

		if err != nil {
			return fmt.Errorf("lint requires an LLM provider: %w", err)
		}

		if r.IsJSON() {
			r.JSON(map[string]any{
				"entries":  len(entries),
				"analysis": analysis,
			})
		} else {
			r.Title("Knowledge Base Lint")
			r.Meta(fmt.Sprintf("Reviewed %d entries", len(entries)))
			r.Divider()
			r.Markdown(analysis)
		}
		return nil
	},
}

func init() {
	wikiCmd.AddCommand(wikiSearchCmd)
	wikiCmd.AddCommand(wikiShowCmd)
	wikiCmd.AddCommand(wikiLsCmd)
	wikiCmd.AddCommand(wikiLintCmd)
	rootCmd.AddCommand(wikiCmd)
}
