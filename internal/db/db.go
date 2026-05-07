package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// DB wraps *sql.DB with driver-aware query helpers.
type DB struct {
	*sql.DB
	driverName string
}

// DriverName returns "sqlite" or "postgres".
func (db *DB) DriverName() string { return db.driverName }

// Rebind converts ? placeholders to $1, $2, ... for PostgreSQL.
// For SQLite it returns the query unchanged.
func (db *DB) Rebind(query string) string {
	if db.driverName != "postgres" {
		return query
	}
	var b strings.Builder
	n := 0
	for _, c := range query {
		if c == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// Like returns LIKE for SQLite and ILIKE for PostgreSQL.
func (db *DB) Like() string {
	if db.driverName == "postgres" {
		return "ILIKE"
	}
	return "LIKE"
}
