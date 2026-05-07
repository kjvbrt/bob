package models

import (
	"fmt"
	"strings"
	"time"
)

// dbHelper provides driver-aware query helpers embedded in each store.
type dbHelper struct {
	driver string
}

func newHelper(driver string) dbHelper {
	return dbHelper{driver: driver}
}

// rebind converts ? placeholders to $1, $2, ... for PostgreSQL.
func (h dbHelper) rebind(query string) string {
	if h.driver != "postgres" {
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

// like returns LIKE for SQLite and ILIKE for PostgreSQL.
func (h dbHelper) like() string {
	if h.driver == "postgres" {
		return "ILIKE"
	}
	return "LIKE"
}

// timeVal is a sql.Scanner that handles both SQLite text timestamps and
// PostgreSQL native time.Time values.
type timeVal struct {
	t *time.Time
}

var timeFormats = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05+00:00",
	"2006-01-02T15:04:05+00:00",
}

func (v timeVal) Scan(src any) error {
	switch s := src.(type) {
	case time.Time:
		*v.t = s
	case string:
		for _, f := range timeFormats {
			if t, err := time.Parse(f, s); err == nil {
				*v.t = t
				return nil
			}
		}
	case []byte:
		return timeVal{v.t}.Scan(string(s))
	}
	return nil
}
