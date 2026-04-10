package cmd

import (
	"fmt"

	"github.com/alexfsong/jeeves/internal/config"
	"github.com/alexfsong/jeeves/internal/llm"
	"github.com/alexfsong/jeeves/internal/render"
	"github.com/alexfsong/jeeves/internal/source"
	"github.com/alexfsong/jeeves/internal/store"
	"github.com/spf13/cobra"
)

var ingestCmd = &cobra.Command{
	Use:   "ingest <url>",
	Short: "Fetch and store content from a URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]
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

		// Resolve topic
		var topicID *int64
		if topicFlag != "" {
			topic, err := st.GetTopicBySlug(topicFlag)
			if err != nil {
				return fmt.Errorf("topic: %w", err)
			}
			topicID = &topic.ID
		}

		// Fetch content
		fetcher := source.NewFetcher()
		content, err := fetcher.Fetch(cmd.Context(), url)
		if err != nil {
			return fmt.Errorf("fetching URL: %w", err)
		}

		// Generate title via LLM if available
		title := url
		router := llm.NewRouter(cfg)
		if router.LocalAvailable() || router.CloudAvailable() {
			snippet := content
			if len(snippet) > 1000 {
				snippet = snippet[:1000]
			}
			generated, err := router.Complete(cmd.Context(), llm.TaskSummarize, llm.Request{
				System: "Generate a concise title (under 100 characters) for this content. Output only the title, nothing else.",
				Prompt: snippet,
				MaxTok: 100,
			})
			if err == nil && generated != "" {
				title = generated
			}
		}

		// Store
		urlStr := url
		k, err := st.AddKnowledge(store.KnowledgeInput{
			TopicID: topicID,
			Type:    store.KnowledgeSource,
			Title:   title,
			Content: content,
			URL:     &urlStr,
			Score:   0.5,
		})
		if err != nil {
			return fmt.Errorf("storing knowledge: %w", err)
		}

		if r.IsJSON() {
			r.JSON(k)
		} else {
			r.Success(fmt.Sprintf("Very good, sir. Content ingested and stored (id: %d).", k.ID))
			r.Body(fmt.Sprintf("  Title: %s", k.Title))
			r.Body(fmt.Sprintf("  Length: %d characters", len(k.Content)))
		}
		return nil
	},
}

func init() {
	ingestCmd.Flags().StringVarP(&topicFlag, "topic", "t", "", "Associate with a topic (by slug)")
	rootCmd.AddCommand(ingestCmd)
}
