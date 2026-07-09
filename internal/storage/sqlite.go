package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"cli_mate/internal/agent"
)

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(ctx context.Context, path string) (*SQLiteStore, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	FOREIGN KEY(session_id) REFERENCES sessions(id)
);`)
	return err
}

func (s *SQLiteStore) CreateSession(ctx context.Context, record SessionRecord) error {
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		record.ID, record.Title, record.CreatedAt, record.UpdatedAt)
	return err
}

func (s *SQLiteStore) AppendMessage(ctx context.Context, sessionID string, message agent.Message) error {
	createdAt := message.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO messages (session_id, role, content, created_at) VALUES (?, ?, ?, ?)`,
		sessionID, string(message.Role), message.Content, createdAt)
	return err
}

func (s *SQLiteStore) Messages(ctx context.Context, sessionID string) ([]agent.Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT role, content, created_at FROM messages WHERE session_id = ? ORDER BY id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []agent.Message
	for rows.Next() {
		var message agent.Message
		var role string
		if err := rows.Scan(&role, &message.Content, &message.CreatedAt); err != nil {
			return nil, err
		}
		message.Role = agent.Role(role)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *SQLiteStore) ListSessions(ctx context.Context) ([]SessionRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, title, created_at, updated_at FROM sessions ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []SessionRecord
	for rows.Next() {
		var rec SessionRecord
		if err := rows.Scan(&rec.ID, &rec.Title, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, rec)
	}
	return sessions, rows.Err()
}

func (s *SQLiteStore) DeleteSession(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE session_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) UpdateSession(ctx context.Context, record SessionRecord) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET title = ?, updated_at = ? WHERE id = ?`,
		record.Title, now, record.ID)
	return err
}

func (s *SQLiteStore) UpdateSessionTitle(ctx context.Context, id string, title string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET title = ?, updated_at = ? WHERE id = ?`,
		title, now, id)
	return err
}
