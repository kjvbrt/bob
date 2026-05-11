package models

import (
	"database/sql"
	"fmt"
	"time"
)

type UpdateType string

const (
	UpdateComment         UpdateType = "comment"
	UpdateInternalNote    UpdateType = "internal_note"
	UpdateStatusChanged   UpdateType = "status_changed"
	UpdatePriorityChanged UpdateType = "priority_changed"
	UpdateAssigned        UpdateType = "assigned"
	UpdateCreated         UpdateType = "created"
)

type Update struct {
	ID          int
	RequestID   int
	UserID      int
	Username    string
	DisplayName string
	Type        UpdateType
	Body        string
	CreatedAt   time.Time
}

func (u *Update) IsComment() bool {
	return u.Type == UpdateComment || u.Type == UpdateInternalNote
}

func (u *Update) IsInternal() bool {
	return u.Type == UpdateInternalNote
}

type UpdateStore struct {
	db *sql.DB
	dbHelper
}

func NewUpdateStore(db *sql.DB, driver string) *UpdateStore {
	return &UpdateStore{db: db, dbHelper: newHelper(driver)}
}

func (us *UpdateStore) Add(requestID, userID int, updateType UpdateType, body string) error {
	var uid interface{}
	if userID != 0 {
		uid = userID
	}
	_, err := us.db.Exec(
		us.rebind(`INSERT INTO request_activity (request_id, user_id, type, body) VALUES (?, ?, ?, ?)`),
		requestID, uid, updateType, body,
	)
	if err != nil {
		return err
	}
	// Touch updated_at so the request floats to the top in activity-sorted views.
	_, err = us.db.Exec(us.rebind(`UPDATE dataset_requests SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`), requestID)
	return err
}

func (us *UpdateStore) GetByRequestID(requestID int) ([]*Update, error) {
	rows, err := us.db.Query(us.rebind(`
		SELECT e.id, e.request_id, COALESCE(e.user_id, 0),
		       COALESCE(u.username, ''), COALESCE(u.display_name, 'System'),
		       e.type, e.body, e.created_at
		FROM request_activity e
		LEFT JOIN users u ON u.id = e.user_id
		WHERE e.request_id = ?
		ORDER BY e.created_at ASC`), requestID)
	if err != nil {
		return nil, fmt.Errorf("query updates: %w", err)
	}
	defer rows.Close()

	var updates []*Update
	for rows.Next() {
		var up Update
		if err := rows.Scan(
			&up.ID, &up.RequestID, &up.UserID,
			&up.Username, &up.DisplayName,
			&up.Type, &up.Body, timeVal{&up.CreatedAt},
		); err != nil {
			return nil, err
		}
		updates = append(updates, &up)
	}
	return updates, rows.Err()
}
