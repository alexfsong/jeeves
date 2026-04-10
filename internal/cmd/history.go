package cmd

import (
	"fmt"

	"github.com/alexfsong/jeeves/internal/render"
	"github.com/alexfsong/jeeves/internal/store"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show research session history",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := render.New(jsonOutput)

		st, err := store.Open()
		if err != nil {
			return err
		}
		defer st.Close()

		var topicID *int64
		topicSlug, _ := cmd.Flags().GetString("topic")
		if topicSlug != "" {
			topic, err := st.GetTopicBySlug(topicSlug)
			if err != nil {
				return fmt.Errorf("topic: %w", err)
			}
			topicID = &topic.ID
		}

		sessions, err := st.ListSessions(topicID, 50)
		if err != nil {
			return err
		}

		if r.IsJSON() {
			r.JSON(sessions)
			return nil
		}

		if len(sessions) == 0 {
			r.Body("No research sessions yet, sir.")
			return nil
		}

		r.Title("Research History")
		r.Divider()
		for _, s := range sessions {
			r.Body(fmt.Sprintf("  %s | %s | %d results | %s",
				s.Timestamp.Format("2006-01-02 15:04"),
				s.Resolution,
				s.ResultCount,
				s.Query,
			))
		}
		return nil
	},
}

func init() {
	historyCmd.Flags().StringP("topic", "t", "", "Filter by topic slug")
	rootCmd.AddCommand(historyCmd)
}
