package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Init(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	database.SetMaxOpenConns(1)

	if err := migrate(database); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return database, nil
}

func migrate(database *sql.DB) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS dataset_requests (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			title           TEXT NOT NULL,
			description     TEXT DEFAULT '',
			requester_name  TEXT NOT NULL,
			requester_email TEXT DEFAULT '',
			department      TEXT DEFAULT '',
			dataset_type    TEXT DEFAULT 'tabular',
			use_case        TEXT DEFAULT '',
			status          TEXT DEFAULT 'pending',
			priority        TEXT DEFAULT 'medium',
			estimated_size  TEXT DEFAULT '',
			format          TEXT DEFAULT '',
			due_date        TEXT DEFAULT '',
			notes           TEXT DEFAULT '',
			tags            TEXT DEFAULT '',
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TRIGGER IF NOT EXISTS update_timestamp
		AFTER UPDATE ON dataset_requests
		BEGIN
			UPDATE dataset_requests SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;
	`)
	return err
}
