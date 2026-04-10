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

var topicCmd = &cobra.Command{
	Use:   "topic",
	Short: "Manage research topics",
}

var topicCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new topic",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.Join(args, " ")
		r := render.New(jsonOutput)

		st, err := store.Open()
		if err != nil {
			return err
		}
		defer st.Close()

		desc, _ := cmd.Flags().GetString("description")
		topic, err := st.CreateTopic(name, desc)
		if err != nil {
			return err
		}

		if r.IsJSON() {
			r.JSON(topic)
		} else {
			r.Success(fmt.Sprintf("Very good, sir. Topic %q created (slug: %s).", topic.Name, topic.Slug))
		}
		return nil
	},
}

var topicLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all topics",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := render.New(jsonOutput)

		st, err := store.Open()
		if err != nil {
			return err
		}
		defer st.Close()

		topics, err := st.ListTopics()
		if err != nil {
			return err
		}

		if r.IsJSON() {
			r.JSON(topics)
			return nil
		}

		if len(topics) == 0 {
			r.Body("No topics yet, sir. Create one with: jeeves topic create \"topic name\"")
			return nil
		}

		r.Title("Topics")
		r.Divider()
		for _, t := range topics {
			count, _ := st.TopicKnowledgeCount(t.ID)
			r.Header(fmt.Sprintf("  %s", t.Name))
			r.Meta(fmt.Sprintf("    slug: %s | entries: %d | updated: %s",
				t.Slug, count, t.UpdatedAt.Format("2006-01-02")))
			if t.Description != "" {
				r.Body(fmt.Sprintf("    %s", t.Description))
			}
		}
		return nil
	},
}

var topicRmCmd = &cobra.Command{
	Use:   "rm <slug>",
	Short: "Remove a topic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r := render.New(jsonOutput)

		st, err := store.Open()
		if err != nil {
			return err
		}
		defer st.Close()

		if err := st.DeleteTopic(args[0]); err != nil {
			return err
		}

		if r.IsJSON() {
			r.JSON(map[string]string{"status": "deleted", "slug": args[0]})
		} else {
			r.Success(fmt.Sprintf("Topic %q removed, sir.", args[0]))
		}
		return nil
	},
}

var topicOutlineCmd = &cobra.Command{
	Use:   "outline <slug>",
	Short: "Show accumulated knowledge tree for a topic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r := render.New(jsonOutput)

		st, err := store.Open()
		if err != nil {
			return err
		}
		defer st.Close()

		topic, err := st.GetTopicBySlug(args[0])
		if err != nil {
			return err
		}

		entries, err := st.ListKnowledgeByTopic(topic.ID)
		if err != nil {
			return err
		}

		if r.IsJSON() {
			r.JSON(map[string]any{
				"topic":   topic,
				"entries": entries,
			})
			return nil
		}

		r.Title(fmt.Sprintf("Outline: %s", topic.Name))
		r.Divider()

		// Group by type
		groups := map[store.KnowledgeType][]store.Knowledge{}
		for _, e := range entries {
			groups[e.Type] = append(groups[e.Type], e)
		}

		for _, kt := range []store.KnowledgeType{store.KnowledgeSynthesis, store.KnowledgeFinding, store.KnowledgeSource, store.KnowledgeNote} {
			items := groups[kt]
			if len(items) == 0 {
				continue
			}
			r.Header(fmt.Sprintf("\n  %s (%d)", strings.Title(string(kt)), len(items)))
			for _, item := range items {
				r.Body(fmt.Sprintf("    • %s", item.Title))
				if item.URL != nil {
					r.URL(fmt.Sprintf("      %s", *item.URL))
				}
				r.Score("relevance", item.Score)
			}
		}

		if len(entries) == 0 {
			r.Body("No knowledge accumulated yet, sir. Try: jeeves research \"query\" --topic " + args[0])
		}
		return nil
	},
}

var topicGapsCmd = &cobra.Command{
	Use:   "gaps <slug>",
	Short: "Identify underexplored areas in a topic",
	Args:  cobra.ExactArgs(1),
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

		topic, err := st.GetTopicBySlug(args[0])
		if err != nil {
			return err
		}

		entries, err := st.ListKnowledgeByTopic(topic.ID)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			if r.IsJSON() {
				r.JSON(map[string]string{"status": "empty", "message": "No knowledge to analyze"})
			} else {
				r.Body("No knowledge accumulated yet, sir. Research some queries first.")
			}
			return nil
		}

		// Build content summary
		var sb strings.Builder
		for _, e := range entries {
			fmt.Fprintf(&sb, "- [%s] %s\n", e.Type, e.Title)
		}

		router := llm.NewRouter(cfg)
		gapRes, _ := cmd.Flags().GetString("resolution")

		var task llm.Task
		if gapRes == "detailed" || gapRes == "full" {
			task = llm.TaskWikiLint // Uses cloud for deeper analysis
		} else {
			task = llm.TaskGapAnalysis
		}

		analysis, err := router.Complete(cmd.Context(), task, llm.Request{
			System: `You are a research gap analyzer. Given a list of knowledge entries for a topic, identify:
1. Subtopics with few entries that need more research
2. Areas where information might be outdated
3. Missing connections between related entries
4. Suggested next research queries

Be concise and actionable. Use markdown.`,
			Prompt: fmt.Sprintf("Topic: %s\n\nKnowledge entries:\n%s", topic.Name, sb.String()),
			MaxTok: 2000,
		})

		if err != nil {
			if r.IsJSON() {
				r.JSON(map[string]any{
					"topic":    topic,
					"entries":  len(entries),
					"analysis": nil,
					"error":    err.Error(),
				})
			} else {
				r.Error(fmt.Sprintf("Unable to analyze gaps: %v", err))
				r.Body("Ensure an LLM provider is available (Ollama or Anthropic API key).")
			}
			return nil
		}

		if r.IsJSON() {
			r.JSON(map[string]any{
				"topic":    topic,
				"entries":  len(entries),
				"analysis": analysis,
			})
		} else {
			r.Title(fmt.Sprintf("Gap Analysis: %s", topic.Name))
			r.Meta(fmt.Sprintf("Based on %d knowledge entries", len(entries)))
			r.Divider()
			r.Markdown(analysis)
		}
		return nil
	},
}

func init() {
	topicCreateCmd.Flags().StringP("description", "d", "", "Topic description")
	topicGapsCmd.Flags().StringP("resolution", "r", "brief", "Analysis depth: brief or detailed")

	topicCmd.AddCommand(topicCreateCmd)
	topicCmd.AddCommand(topicLsCmd)
	topicCmd.AddCommand(topicRmCmd)
	topicCmd.AddCommand(topicOutlineCmd)
	topicCmd.AddCommand(topicGapsCmd)
	rootCmd.AddCommand(topicCmd)
}
