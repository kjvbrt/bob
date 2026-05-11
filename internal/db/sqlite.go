//go:build !prod

package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Init() (*DB, error) {
	path := os.Getenv("SQLITE_PATH")
	if path == "" {
		path = "./data/requests.db"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	sqldb.SetMaxOpenConns(1)

	db := &DB{DB: sqldb, driverName: "sqlite"}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func migrate(db *DB) error {
	// Additive migrations — errors swallowed (column/table already exists).
	db.Exec(`ALTER TABLE dataset_requests ADD COLUMN created_by INTEGER REFERENCES users(id)`)
	db.Exec(`ALTER TABLE dataset_requests ADD COLUMN requester_username TEXT NOT NULL DEFAULT ''`)
	db.Exec(`ALTER TABLE dataset_requests ADD COLUMN assigned_to INTEGER REFERENCES users(id)`)
	db.Exec(`ALTER TABLE dataset_requests RENAME COLUMN requester_cern_username TO requester_username`)
	db.Exec(`ALTER TABLE users RENAME COLUMN cern_username TO username`)
	db.Exec(`ALTER TABLE dataset_requests ADD COLUMN physics_approval TEXT NOT NULL DEFAULT ''`)
	db.Exec(`ALTER TABLE dataset_requests ADD COLUMN resources_approval TEXT NOT NULL DEFAULT ''`)
	db.Exec(`ALTER TABLE dataset_requests ADD COLUMN statistics TEXT NOT NULL DEFAULT ''`)
	db.Exec(`ALTER TABLE dataset_requests ADD COLUMN target_campaign TEXT NOT NULL DEFAULT ''`)
	db.Exec(`ALTER TABLE dataset_requests ADD COLUMN key4hep_stack TEXT NOT NULL DEFAULT ''`)

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			username     TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL DEFAULT '',
			email        TEXT NOT NULL DEFAULT '',
			role         TEXT NOT NULL DEFAULT 'requester',
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login   DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id         TEXT PRIMARY KEY,
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS dataset_requests (
			id                 INTEGER PRIMARY KEY AUTOINCREMENT,
			title              TEXT NOT NULL,
			description        TEXT DEFAULT '',
			requester_name     TEXT NOT NULL,
			requester_username TEXT NOT NULL DEFAULT '',
			requester_email    TEXT DEFAULT '',
			department         TEXT DEFAULT '',
			dataset_type       TEXT DEFAULT 'simulation',
			use_case           TEXT DEFAULT 'physics_analysis',
			status             TEXT DEFAULT 'pending',
			priority           TEXT DEFAULT 'medium',
			estimated_size     TEXT DEFAULT '',
			statistics         TEXT DEFAULT '',
			target_campaign    TEXT DEFAULT '',
			key4hep_stack      TEXT DEFAULT '',
			format             TEXT DEFAULT '',
			due_date           TEXT DEFAULT '',
			notes              TEXT DEFAULT '',
			tags               TEXT DEFAULT '',
			created_by         INTEGER REFERENCES users(id),
			assigned_to        INTEGER REFERENCES users(id),
			physics_approval   TEXT NOT NULL DEFAULT '',
			resources_approval TEXT NOT NULL DEFAULT '',
			created_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at         DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS request_activity (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id INTEGER NOT NULL REFERENCES dataset_requests(id) ON DELETE CASCADE,
			user_id    INTEGER REFERENCES users(id),
			type       TEXT NOT NULL DEFAULT 'comment',
			body       TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS request_relations (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			from_id    INTEGER NOT NULL REFERENCES dataset_requests(id) ON DELETE CASCADE,
			to_id      INTEGER NOT NULL REFERENCES dataset_requests(id) ON DELETE CASCADE,
			type       TEXT NOT NULL DEFAULT 'related',
			created_by INTEGER REFERENCES users(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(from_id, to_id, type)
		);

		CREATE TRIGGER IF NOT EXISTS update_timestamp
		AFTER UPDATE ON dataset_requests
		BEGIN
			UPDATE dataset_requests SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;
	`)
	return err
}
