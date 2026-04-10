package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexfsong/jeeves/internal/config"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open() (*Store, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dir, "jeeves.db")

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS topics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			created_at DATETIME DEFAULT (datetime('now')),
			updated_at DATETIME DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic_id INTEGER REFERENCES topics(id) ON DELETE SET NULL,
			type TEXT NOT NULL CHECK(type IN ('finding','synthesis','source','note')),
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			url TEXT UNIQUE,
			score REAL DEFAULT 0.0,
			created_at DATETIME DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_fts USING fts5(
			title, content, content=knowledge, content_rowid=id
		)`,
		// Triggers to keep FTS in sync
		`CREATE TRIGGER IF NOT EXISTS knowledge_ai AFTER INSERT ON knowledge BEGIN
			INSERT INTO knowledge_fts(rowid, title, content) VALUES (new.id, new.title, new.content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS knowledge_ad AFTER DELETE ON knowledge BEGIN
			INSERT INTO knowledge_fts(knowledge_fts, rowid, title, content) VALUES('delete', old.id, old.title, old.content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS knowledge_au AFTER UPDATE ON knowledge BEGIN
			INSERT INTO knowledge_fts(knowledge_fts, rowid, title, content) VALUES('delete', old.id, old.title, old.content);
			INSERT INTO knowledge_fts(rowid, title, content) VALUES (new.id, new.title, new.content);
		END`,
		`CREATE TABLE IF NOT EXISTS knowledge_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_id INTEGER NOT NULL REFERENCES knowledge(id) ON DELETE CASCADE,
			to_id INTEGER NOT NULL REFERENCES knowledge(id) ON DELETE CASCADE,
			kind TEXT NOT NULL CHECK(kind IN ('related','subtopic','contradicts','supersedes')),
			UNIQUE(from_id, to_id, kind)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic_id INTEGER REFERENCES topics(id) ON DELETE SET NULL,
			query TEXT NOT NULL,
			resolution TEXT NOT NULL,
			result_count INTEGER DEFAULT 0,
			timestamp DATETIME DEFAULT (datetime('now'))
		)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}
	return nil
}
