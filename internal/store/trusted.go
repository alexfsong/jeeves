package store

import (
	"database/sql"
	"fmt"
	"time"
)

type TrustedSource struct {
	ID         int64     `json:"id"`
	Domain     string    `json:"domain"`
	TrustLevel float64   `json:"trust_level"`
	TopicID    *int64    `json:"topic_id,omitempty"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Store) AddTrustedSource(domain string, trustLevel float64, topicID *int64, notes string) (*TrustedSource, error) {
	res, err := s.db.Exec(
		"INSERT INTO trusted_sources (domain, trust_level, topic_id, notes) VALUES (?, ?, ?, ?)",
		domain, trustLevel, topicID, notes,
	)
	if err != nil {
		return nil, fmt.Errorf("adding trusted source: %w", err)
	}

	id, _ := res.LastInsertId()
	return &TrustedSource{
		ID:         id,
		Domain:     domain,
		TrustLevel: trustLevel,
		TopicID:    topicID,
		Notes:      notes,
		CreatedAt:  time.Now(),
	}, nil
}

// ListTrustedSources returns trusted sources. If topicID is non-nil, returns
// both global sources (topic_id IS NULL) and topic-specific sources.
func (s *Store) ListTrustedSources(topicID *int64) ([]TrustedSource, error) {
	var query string
	var args []any

	if topicID != nil {
		query = `SELECT id, domain, trust_level, topic_id, notes, created_at
				 FROM trusted_sources
				 WHERE topic_id IS NULL OR topic_id = ?
				 ORDER BY domain`
		args = []any{*topicID}
	} else {
		query = `SELECT id, domain, trust_level, topic_id, notes, created_at
				 FROM trusted_sources
				 ORDER BY domain`
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []TrustedSource
	for rows.Next() {
		var ts TrustedSource
		if err := rows.Scan(&ts.ID, &ts.Domain, &ts.TrustLevel, &ts.TopicID, &ts.Notes, &ts.CreatedAt); err != nil {
			return nil, err
		}
		sources = append(sources, ts)
	}
	return sources, rows.Err()
}

// ListAllTrustedSources returns every trusted source in the database.
func (s *Store) ListAllTrustedSources() ([]TrustedSource, error) {
	return s.ListTrustedSources(nil)
}

func (s *Store) RemoveTrustedSource(id int64) error {
	res, err := s.db.Exec("DELETE FROM trusted_sources WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("trusted source %d not found", id)
	}
	return nil
}

// GetTrustForDomain returns the effective trust level for a domain.
// Per-topic overrides global. Returns 1.0 (neutral) if no match.
func (s *Store) GetTrustForDomain(domain string, topicID *int64) float64 {
	// Try topic-specific first
	if topicID != nil {
		var level float64
		err := s.db.QueryRow(
			"SELECT trust_level FROM trusted_sources WHERE domain = ? AND topic_id = ?",
			domain, *topicID,
		).Scan(&level)
		if err == nil {
			return level
		}
	}

	// Fall back to global
	var level float64
	err := s.db.QueryRow(
		"SELECT trust_level FROM trusted_sources WHERE domain = ? AND topic_id IS NULL",
		domain,
	).Scan(&level)
	if err == nil {
		return level
	}

	return 1.0
}

// UpdateTrustedSource updates the trust level and notes of an existing source.
func (s *Store) UpdateTrustedSource(id int64, trustLevel float64, notes string) error {
	res, err := s.db.Exec(
		"UPDATE trusted_sources SET trust_level = ?, notes = ? WHERE id = ?",
		trustLevel, notes, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("trusted source %d not found", id)
	}
	return nil
}

// GetTrustedSource returns a single trusted source by ID.
func (s *Store) GetTrustedSource(id int64) (*TrustedSource, error) {
	ts := &TrustedSource{}
	err := s.db.QueryRow(
		"SELECT id, domain, trust_level, topic_id, notes, created_at FROM trusted_sources WHERE id = ?",
		id,
	).Scan(&ts.ID, &ts.Domain, &ts.TrustLevel, &ts.TopicID, &ts.Notes, &ts.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("trusted source %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	return ts, nil
}
