package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Status string
type Priority string

const (
	StatusDraft      Status = "draft"
	StatusPending    Status = "pending"
	StatusApproved   Status = "approved"
	StatusRejected   Status = "rejected"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusCancelled  Status = "cancelled"
)

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

type DatasetRequest struct {
	ID             int
	Title          string
	Description    string
	RequesterName  string
	RequesterEmail string
	Department     string
	DatasetType    string
	UseCase        string
	Status         Status
	Priority       Priority
	EstimatedSize  string
	Format         string
	DueDate        string
	Notes          string
	Tags           string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (r *DatasetRequest) StatusLabel() string {
	switch r.Status {
	case StatusDraft:
		return "Draft"
	case StatusPending:
		return "Pending Review"
	case StatusApproved:
		return "Approved"
	case StatusRejected:
		return "Rejected"
	case StatusInProgress:
		return "In Progress"
	case StatusCompleted:
		return "Completed"
	case StatusCancelled:
		return "Cancelled"
	default:
		return string(r.Status)
	}
}

func (r *DatasetRequest) PriorityLabel() string {
	switch r.Priority {
	case PriorityLow:
		return "Low"
	case PriorityMedium:
		return "Medium"
	case PriorityHigh:
		return "High"
	case PriorityCritical:
		return "Critical"
	default:
		return string(r.Priority)
	}
}

func (r *DatasetRequest) TagList() []string {
	if r.Tags == "" {
		return nil
	}
	parts := strings.Split(r.Tags, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

type Stats struct {
	Total      int
	Pending    int
	InProgress int
	Completed  int
	Critical   int
	Rejected   int
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetAll(status, priority, search string) ([]*DatasetRequest, error) {
	query := `SELECT id, title, description, requester_name, requester_email,
		department, dataset_type, use_case, status, priority, estimated_size,
		format, due_date, notes, tags, created_at, updated_at
		FROM dataset_requests WHERE 1=1`

	args := []interface{}{}

	if status != "" && status != "all" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if priority != "" && priority != "all" {
		query += " AND priority = ?"
		args = append(args, priority)
	}
	if search != "" {
		query += " AND (title LIKE ? OR description LIKE ? OR requester_name LIKE ? OR department LIKE ?)"
		s := "%" + search + "%"
		args = append(args, s, s, s, s)
	}

	query += ` ORDER BY
		CASE priority WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
		CASE status WHEN 'in_progress' THEN 0 WHEN 'pending' THEN 1 WHEN 'approved' THEN 2 ELSE 3 END,
		created_at DESC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query requests: %w", err)
	}
	defer rows.Close()

	var requests []*DatasetRequest
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, rows.Err()
}

func (r *Repository) GetByID(id int) (*DatasetRequest, error) {
	row := r.db.QueryRow(`SELECT id, title, description, requester_name, requester_email,
		department, dataset_type, use_case, status, priority, estimated_size,
		format, due_date, notes, tags, created_at, updated_at
		FROM dataset_requests WHERE id = ?`, id)
	return scanRequest(row)
}

func (r *Repository) Create(req *DatasetRequest) (int64, error) {
	result, err := r.db.Exec(`
		INSERT INTO dataset_requests
			(title, description, requester_name, requester_email, department,
			 dataset_type, use_case, status, priority, estimated_size, format,
			 due_date, notes, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Title, req.Description, req.RequesterName, req.RequesterEmail,
		req.Department, req.DatasetType, req.UseCase, req.Status, req.Priority,
		req.EstimatedSize, req.Format, req.DueDate, req.Notes, req.Tags,
	)
	if err != nil {
		return 0, fmt.Errorf("insert request: %w", err)
	}
	return result.LastInsertId()
}

func (r *Repository) Update(req *DatasetRequest) error {
	_, err := r.db.Exec(`
		UPDATE dataset_requests SET
			title=?, description=?, requester_name=?, requester_email=?,
			department=?, dataset_type=?, use_case=?, status=?, priority=?,
			estimated_size=?, format=?, due_date=?, notes=?, tags=?
		WHERE id=?`,
		req.Title, req.Description, req.RequesterName, req.RequesterEmail,
		req.Department, req.DatasetType, req.UseCase, req.Status, req.Priority,
		req.EstimatedSize, req.Format, req.DueDate, req.Notes, req.Tags, req.ID,
	)
	return err
}

func (r *Repository) UpdateStatus(id int, status Status) error {
	_, err := r.db.Exec("UPDATE dataset_requests SET status=? WHERE id=?", status, id)
	return err
}

func (r *Repository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM dataset_requests WHERE id=?", id)
	return err
}

func (r *Repository) GetStats() (*Stats, error) {
	var stats Stats
	row := r.db.QueryRow(`
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN status = 'pending'     THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'completed'   THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN priority = 'critical'  THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'rejected'    THEN 1 ELSE 0 END), 0)
		FROM dataset_requests
	`)
	err := row.Scan(
		&stats.Total, &stats.Pending, &stats.InProgress,
		&stats.Completed, &stats.Critical, &stats.Rejected,
	)
	return &stats, err
}

func (r *Repository) GetRecent(limit int) ([]*DatasetRequest, error) {
	rows, err := r.db.Query(`
		SELECT id, title, description, requester_name, requester_email,
		department, dataset_type, use_case, status, priority, estimated_size,
		format, due_date, notes, tags, created_at, updated_at
		FROM dataset_requests ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []*DatasetRequest
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, rows.Err()
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanRequest(row scannable) (*DatasetRequest, error) {
	var req DatasetRequest
	var createdAt, updatedAt string
	err := row.Scan(
		&req.ID, &req.Title, &req.Description, &req.RequesterName, &req.RequesterEmail,
		&req.Department, &req.DatasetType, &req.UseCase, &req.Status, &req.Priority,
		&req.EstimatedSize, &req.Format, &req.DueDate, &req.Notes, &req.Tags,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	req.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	req.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &req, nil
}
