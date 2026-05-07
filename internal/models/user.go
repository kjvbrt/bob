package models

import (
	"database/sql"
	"fmt"
	"time"
)

type Role string

const (
	RoleRequester Role = "requester"
	RoleManager   Role = "manager"
)

type User struct {
	ID          int
	Username    string // preferred_username from OIDC
	DisplayName string
	Email       string
	Role        Role
	CreatedAt   time.Time
	LastLogin   time.Time
}

func (u *User) IsManager() bool {
	if u == nil {
		return false
	}
	return u.Role == RoleManager
}

func (u *User) IsRequester() bool {
	if u == nil {
		return false
	}
	return u.Role == RoleRequester
}

// Initial returns the first character of the display name, safe for nil.
func (u *User) Initial() string {
	if u == nil || len(u.DisplayName) == 0 {
		return "?"
	}
	return string([]rune(u.DisplayName)[0])
}

type UserStore struct {
	db *sql.DB
	dbHelper
}

func NewUserStore(db *sql.DB, driver string) *UserStore {
	return &UserStore{db: db, dbHelper: newHelper(driver)}
}

// Upsert creates or updates a user on every SSO login, returning the current record.
func (r *UserStore) Upsert(username, displayName, email string, role Role) (*User, error) {
	_, err := r.db.Exec(r.rebind(`
		INSERT INTO users (username, display_name, email, role, last_login)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(username) DO UPDATE SET
			display_name = excluded.display_name,
			email        = excluded.email,
			last_login   = CURRENT_TIMESTAMP
	`), username, displayName, email, role)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return r.GetByUsername(username)
}

func (r *UserStore) GetByUsername(username string) (*User, error) {
	row := r.db.QueryRow(r.rebind(`
		SELECT id, username, display_name, email, role, created_at, last_login
		FROM users WHERE username = ?`), username)
	return scanUser(row)
}

func (r *UserStore) GetByID(id int) (*User, error) {
	row := r.db.QueryRow(r.rebind(`
		SELECT id, username, display_name, email, role, created_at, last_login
		FROM users WHERE id = ?`), id)
	return scanUser(row)
}

// Sessions

func (r *UserStore) CreateSession(userID int, token string, expiresAt time.Time) error {
	_, err := r.db.Exec(
		r.rebind(`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`),
		token, userID, expiresAt,
	)
	return err
}

func (r *UserStore) GetSession(token string) (*User, error) {
	row := r.db.QueryRow(r.rebind(`
		SELECT u.id, u.username, u.display_name, u.email, u.role, u.created_at, u.last_login
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = ? AND s.expires_at > CURRENT_TIMESTAMP
	`), token)
	return scanUser(row)
}

func (r *UserStore) DeleteSession(token string) error {
	_, err := r.db.Exec(r.rebind(`DELETE FROM sessions WHERE id = ?`), token)
	return err
}

func (r *UserStore) GetManagers() ([]*User, error) {
	rows, err := r.db.Query(`
		SELECT id, username, display_name, email, role, created_at, last_login
		FROM users WHERE role = 'manager' ORDER BY display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var managers []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		managers = append(managers, u)
	}
	return managers, rows.Err()
}

func (r *UserStore) PurgeExpiredSessions() error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP`)
	return err
}

func scanUser(row scannable) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Role,
		timeVal{&u.CreatedAt}, timeVal{&u.LastLogin},
	)
	return &u, err
}
