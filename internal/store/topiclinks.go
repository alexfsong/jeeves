package store

import "time"

type TopicLink struct {
	ID        int64     `json:"id"`
	FromID    int64     `json:"from_id"`
	ToID      int64     `json:"to_id"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) AddTopicLink(fromID, toID int64, kind string) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO topic_links (from_id, to_id, kind) VALUES (?, ?, ?)",
		fromID, toID, kind,
	)
	return err
}

func (s *Store) ListTopicLinks() ([]TopicLink, error) {
	rows, err := s.db.Query(
		"SELECT id, from_id, to_id, kind, created_at FROM topic_links ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []TopicLink
	for rows.Next() {
		var l TopicLink
		if err := rows.Scan(&l.ID, &l.FromID, &l.ToID, &l.Kind, &l.CreatedAt); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// GetTopicBranches returns topics that branched from the given topic.
func (s *Store) GetTopicBranches(topicID int64) ([]Topic, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.slug, t.name, t.description, t.created_at, t.updated_at
		 FROM topics t
		 JOIN topic_links tl ON tl.from_id = t.id
		 WHERE tl.to_id = ? AND tl.kind = 'branched_from'
		 ORDER BY t.created_at DESC`,
		topicID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

// GetBranchAncestors walks the branched_from chain upward, returning the research stack.
func (s *Store) GetBranchAncestors(topicID int64) ([]Topic, error) {
	var ancestors []Topic
	currentID := topicID
	seen := make(map[int64]bool)

	for {
		if seen[currentID] {
			break // cycle protection
		}
		seen[currentID] = true

		var parentID int64
		err := s.db.QueryRow(
			`SELECT tl.to_id FROM topic_links tl
			 WHERE tl.from_id = ? AND tl.kind = 'branched_from'
			 LIMIT 1`,
			currentID,
		).Scan(&parentID)
		if err != nil {
			break // no parent or error — end of chain
		}

		t := &Topic{}
		err = s.db.QueryRow(
			"SELECT id, slug, name, description, created_at, updated_at FROM topics WHERE id = ?",
			parentID,
		).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			break
		}

		ancestors = append(ancestors, *t)
		currentID = parentID
	}

	return ancestors, nil
}
