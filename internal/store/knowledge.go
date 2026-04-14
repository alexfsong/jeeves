package store

import (
	"database/sql"
	"fmt"
	"time"
)

type KnowledgeType string

const (
	KnowledgeFinding        KnowledgeType = "finding"
	KnowledgeSynthesis      KnowledgeType = "synthesis"
	KnowledgeSource         KnowledgeType = "source"
	KnowledgeVerifiedSource KnowledgeType = "verified_source"
	KnowledgeNote           KnowledgeType = "note"
)

type Knowledge struct {
	ID        int64         `json:"id"`
	TopicID   *int64        `json:"topic_id,omitempty"`
	Type      KnowledgeType `json:"type"`
	Title     string        `json:"title"`
	Content   string        `json:"content"`
	URL       *string       `json:"url,omitempty"`
	Score     float64       `json:"score"`
	CreatedAt time.Time     `json:"created_at"`
}

type KnowledgeInput struct {
	TopicID *int64
	Type    KnowledgeType
	Title   string
	Content string
	URL     *string
	Score   float64
}

func (s *Store) AddKnowledge(k KnowledgeInput) (*Knowledge, error) {
	// Dedup by URL
	if k.URL != nil && *k.URL != "" {
		var existingID int64
		err := s.db.QueryRow("SELECT id FROM knowledge WHERE url = ?", *k.URL).Scan(&existingID)
		if err == nil {
			// Already exists, return existing
			return s.GetKnowledge(existingID)
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	res, err := s.db.Exec(
		"INSERT INTO knowledge (topic_id, type, title, content, url, score) VALUES (?, ?, ?, ?, ?, ?)",
		k.TopicID, string(k.Type), k.Title, k.Content, k.URL, k.Score,
	)
	if err != nil {
		return nil, fmt.Errorf("adding knowledge: %w", err)
	}

	id, _ := res.LastInsertId()

	if k.TopicID != nil {
		s.TouchTopic(*k.TopicID)
	}

	return &Knowledge{
		ID:        id,
		TopicID:   k.TopicID,
		Type:      k.Type,
		Title:     k.Title,
		Content:   k.Content,
		URL:       k.URL,
		Score:     k.Score,
		CreatedAt: time.Now(),
	}, nil
}

func (s *Store) GetKnowledge(id int64) (*Knowledge, error) {
	k := &Knowledge{}
	err := s.db.QueryRow(
		"SELECT id, topic_id, type, title, content, url, score, created_at FROM knowledge WHERE id = ?",
		id,
	).Scan(&k.ID, &k.TopicID, &k.Type, &k.Title, &k.Content, &k.URL, &k.Score, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (s *Store) SearchKnowledge(query string, limit int) ([]Knowledge, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT k.id, k.topic_id, k.type, k.title, k.content, k.url, k.score, k.created_at
		 FROM knowledge_fts fts
		 JOIN knowledge k ON k.id = fts.rowid
		 WHERE knowledge_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("searching knowledge: %w", err)
	}
	defer rows.Close()

	return scanKnowledgeRows(rows)
}

func (s *Store) ListKnowledgeByTopic(topicID int64) ([]Knowledge, error) {
	rows, err := s.db.Query(
		`SELECT id, topic_id, type, title, content, url, score, created_at
		 FROM knowledge WHERE topic_id = ? ORDER BY score DESC, created_at DESC`,
		topicID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanKnowledgeRows(rows)
}

func (s *Store) ListAllKnowledge(limit int) ([]Knowledge, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, topic_id, type, title, content, url, score, created_at
		 FROM knowledge ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanKnowledgeRows(rows)
}

func (s *Store) AddKnowledgeLink(fromID, toID int64, kind string) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO knowledge_links (from_id, to_id, kind) VALUES (?, ?, ?)",
		fromID, toID, kind,
	)
	return err
}

func scanKnowledgeRows(rows *sql.Rows) ([]Knowledge, error) {
	var results []Knowledge
	for rows.Next() {
		var k Knowledge
		if err := rows.Scan(&k.ID, &k.TopicID, &k.Type, &k.Title, &k.Content, &k.URL, &k.Score, &k.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, k)
	}
	return results, rows.Err()
}
