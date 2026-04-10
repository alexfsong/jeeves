package store

import "time"

type Session struct {
	ID          int64     `json:"id"`
	TopicID     *int64    `json:"topic_id,omitempty"`
	Query       string    `json:"query"`
	Resolution  string    `json:"resolution"`
	ResultCount int       `json:"result_count"`
	Timestamp   time.Time `json:"timestamp"`
}

func (s *Store) LogSession(topicID *int64, query, resolution string, resultCount int) error {
	_, err := s.db.Exec(
		"INSERT INTO sessions (topic_id, query, resolution, result_count) VALUES (?, ?, ?, ?)",
		topicID, query, resolution, resultCount,
	)
	return err
}

func (s *Store) ListSessions(topicID *int64, limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 50
	}

	var query string
	var args []any
	if topicID != nil {
		query = `SELECT id, topic_id, query, resolution, result_count, timestamp
				 FROM sessions WHERE topic_id = ? ORDER BY timestamp DESC LIMIT ?`
		args = []any{*topicID, limit}
	} else {
		query = `SELECT id, topic_id, query, resolution, result_count, timestamp
				 FROM sessions ORDER BY timestamp DESC LIMIT ?`
		args = []any{limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.TopicID, &sess.Query, &sess.Resolution, &sess.ResultCount, &sess.Timestamp); err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}
