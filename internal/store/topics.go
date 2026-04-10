package store

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Topic struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func slugify(name string) string {
	s := strings.ToLower(name)
	s = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`[\s]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func (s *Store) CreateTopic(name, description string) (*Topic, error) {
	slug := slugify(name)

	// Dedup slug
	base := slug
	for i := 2; ; i++ {
		var exists int
		err := s.db.QueryRow("SELECT COUNT(*) FROM topics WHERE slug = ?", slug).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if exists == 0 {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}

	res, err := s.db.Exec(
		"INSERT INTO topics (slug, name, description) VALUES (?, ?, ?)",
		slug, name, description,
	)
	if err != nil {
		return nil, fmt.Errorf("creating topic: %w", err)
	}

	id, _ := res.LastInsertId()
	return &Topic{
		ID:          id,
		Slug:        slug,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (s *Store) GetTopicBySlug(slug string) (*Topic, error) {
	t := &Topic{}
	err := s.db.QueryRow(
		"SELECT id, slug, name, description, created_at, updated_at FROM topics WHERE slug = ?",
		slug,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("topic %q not found", slug)
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) ListTopics() ([]Topic, error) {
	rows, err := s.db.Query(
		"SELECT id, slug, name, description, created_at, updated_at FROM topics ORDER BY updated_at DESC",
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

func (s *Store) DeleteTopic(slug string) error {
	res, err := s.db.Exec("DELETE FROM topics WHERE slug = ?", slug)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("topic %q not found", slug)
	}
	return nil
}

func (s *Store) TouchTopic(id int64) error {
	_, err := s.db.Exec("UPDATE topics SET updated_at = datetime('now') WHERE id = ?", id)
	return err
}

func (s *Store) TopicKnowledgeCount(topicID int64) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM knowledge WHERE topic_id = ?", topicID).Scan(&count)
	return count, err
}
