//go:build prod

package db

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Init() (*DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := sqldb.Ping(); err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	db := &DB{DB: sqldb, driverName: "postgres"}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func migrate(db *DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id           SERIAL PRIMARY KEY,
			username     TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL DEFAULT '',
			email        TEXT NOT NULL DEFAULT '',
			role         TEXT NOT NULL DEFAULT 'requester',
			created_at   TIMESTAMPTZ DEFAULT NOW(),
			last_login   TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id         TEXT PRIMARY KEY,
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS dataset_requests (
			id                 SERIAL PRIMARY KEY,
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
			format             TEXT DEFAULT '',
			due_date           TEXT DEFAULT '',
			notes              TEXT DEFAULT '',
			tags               TEXT DEFAULT '',
			created_by         INTEGER REFERENCES users(id),
			assigned_to        INTEGER REFERENCES users(id),
			physics_approval   TEXT NOT NULL DEFAULT '',
			resources_approval TEXT NOT NULL DEFAULT '',
			created_at         TIMESTAMPTZ DEFAULT NOW(),
			updated_at         TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS request_events (
			id         SERIAL PRIMARY KEY,
			request_id INTEGER NOT NULL REFERENCES dataset_requests(id) ON DELETE CASCADE,
			user_id    INTEGER REFERENCES users(id),
			type       TEXT NOT NULL DEFAULT 'comment',
			body       TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE OR REPLACE FUNCTION update_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql`,
		`DROP TRIGGER IF EXISTS update_timestamp ON dataset_requests`,
		`CREATE TRIGGER update_timestamp
			BEFORE UPDATE ON dataset_requests
			FOR EACH ROW EXECUTE FUNCTION update_updated_at()`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}
