package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Status string
type Priority string

type Option struct {
	Value string
	Label string
}

var UseCaseLabels = []Option{
	{"physics_analysis", "Physics Analysis"},
	{"reconstruction_dev", "Reconstruction Development"},
	{"detector_simulation", "Detector Simulation"},
	{"ml_training", "ML Training"},
	{"ml_evaluation", "ML Evaluation"},
	{"benchmarking", "Benchmarking"},
	{"calibration", "Calibration"},
	{"other", "Other"},
}

var DatasetTypeLabels = []Option{
	{"generation", "Generation"},
	{"simulation", "Simulation"},
	{"reconstruction", "Reconstruction"},
	{"other", "Other"},
}

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
	ID                    int
	Title                 string
	Description           string
	RequesterName         string
	RequesterUsername     string
	RequesterEmail        string
	Department            string
	DatasetType           string
	UseCase               string
	Status                Status
	Priority              Priority
	EstimatedSize         string
	Format                string
	DueDate               string
	Notes                 string
	Tags                  string
	CreatedBy             int
	AssignedTo            int
	AssignedToName        string
	PhysicsApproval       string // "" | "approved" | "rejected"
	ResourcesApproval     string // "" | "approved" | "rejected"
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (r *DatasetRequest) ApprovalLabel(v string) string {
	switch v {
	case "approved":
		return "Approved"
	case "rejected":
		return "Rejected"
	default:
		return "Pending"
	}
}

func (r *DatasetRequest) StatusLabel() string {
	switch r.Status {
	case StatusDraft:
		return "Draft"
	case StatusPending:
		return "Under Review"
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

func (r *DatasetRequest) UseCaseLabel() string {
	for _, o := range UseCaseLabels {
		if o.Value == r.UseCase {
			return o.Label
		}
	}
	return r.UseCase
}

func (r *DatasetRequest) DatasetTypeLabel() string {
	for _, o := range DatasetTypeLabels {
		if o.Value == r.DatasetType {
			return o.Label
		}
	}
	return r.DatasetType
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

type RequestStore struct {
	db *sql.DB
}

func NewRequestStore(db *sql.DB) *RequestStore {
	return &RequestStore{db: db}
}

const selectCols = `
	dr.id, dr.title, dr.description, dr.requester_name, dr.requester_username, dr.requester_email,
	dr.department, dr.dataset_type, dr.use_case, dr.status, dr.priority, dr.estimated_size,
	dr.format, dr.due_date, dr.notes, dr.tags, COALESCE(dr.created_by,0),
	COALESCE(dr.assigned_to,0), COALESCE(au.display_name,''),
	COALESCE(dr.physics_approval,''), COALESCE(dr.resources_approval,''),
	dr.created_at, dr.updated_at`

func requestOrderBy(col, dir string) string {
	d := "DESC"
	if strings.ToLower(dir) == "asc" {
		d = "ASC"
	}
	switch col {
	case "title":
		return "dr.title " + d
	case "requester":
		return "dr.requester_name " + d
	case "status":
		return "CASE dr.status WHEN 'in_progress' THEN 0 WHEN 'pending' THEN 1 WHEN 'approved' THEN 2 WHEN 'draft' THEN 3 WHEN 'completed' THEN 4 WHEN 'rejected' THEN 5 ELSE 6 END " + d
	case "priority":
		return "CASE dr.priority WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END " + d
	case "created":
		return "dr.created_at " + d
	case "updated":
		return "dr.updated_at " + d
	default:
		return `dr.updated_at DESC`
	}
}

func (r *RequestStore) GetAll(status, priority, search, sortCol, sortDir string, page, perPage int) ([]*DatasetRequest, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}

	if status != "" && status != "all" {
		where += " AND dr.status = ?"
		args = append(args, status)
	}
	if priority != "" && priority != "all" {
		where += " AND dr.priority = ?"
		args = append(args, priority)
	}
	if search != "" {
		where += " AND (dr.title LIKE ? OR dr.description LIKE ? OR dr.requester_name LIKE ? OR dr.department LIKE ?)"
		s := "%" + search + "%"
		args = append(args, s, s, s, s)
	}

	var total int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM dataset_requests dr "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count requests: %w", err)
	}

	if perPage <= 0 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}

	query := `SELECT` + selectCols + `
		FROM dataset_requests dr
		LEFT JOIN users au ON au.id = dr.assigned_to
		` + where + ` ORDER BY ` + requestOrderBy(sortCol, sortDir) + ` LIMIT ? OFFSET ?`
	args = append(args, perPage, (page-1)*perPage)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query requests: %w", err)
	}
	defer rows.Close()

	var requests []*DatasetRequest
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, 0, err
		}
		requests = append(requests, req)
	}
	return requests, total, rows.Err()
}

// GetActive returns non-terminal requests (pending, approved, in_progress) sorted by priority then age.
func (r *RequestStore) GetActive() ([]*DatasetRequest, error) {
	rows, err := r.db.Query(`SELECT` + selectCols + `
		FROM dataset_requests dr
		LEFT JOIN users au ON au.id = dr.assigned_to
		WHERE dr.status IN ('pending', 'approved', 'in_progress')
		ORDER BY
			CASE dr.priority WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
			dr.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query active requests: %w", err)
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

func (r *RequestStore) GetByID(id int) (*DatasetRequest, error) {
	row := r.db.QueryRow(`SELECT`+selectCols+`
		FROM dataset_requests dr
		LEFT JOIN users au ON au.id = dr.assigned_to
		WHERE dr.id = ?`, id)
	return scanRequest(row)
}

func (r *RequestStore) Create(req *DatasetRequest) (int64, error) {
	var createdBy interface{}
	if req.CreatedBy != 0 {
		createdBy = req.CreatedBy
	}
	result, err := r.db.Exec(`
		INSERT INTO dataset_requests
			(title, description, requester_name, requester_username, requester_email,
			 department, dataset_type, use_case, status, priority, estimated_size, format,
			 due_date, notes, tags, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Title, req.Description, req.RequesterName, req.RequesterUsername, req.RequesterEmail,
		req.Department, req.DatasetType, req.UseCase, req.Status, req.Priority,
		req.EstimatedSize, req.Format, req.DueDate, req.Notes, req.Tags, createdBy,
	)
	if err != nil {
		return 0, fmt.Errorf("insert request: %w", err)
	}
	return result.LastInsertId()
}

func (r *RequestStore) Update(req *DatasetRequest) error {
	_, err := r.db.Exec(`
		UPDATE dataset_requests SET
			title=?, description=?, requester_name=?, requester_username=?, requester_email=?,
			department=?, dataset_type=?, use_case=?, status=?, priority=?,
			estimated_size=?, format=?, due_date=?, notes=?, tags=?
		WHERE id=?`,
		req.Title, req.Description, req.RequesterName, req.RequesterUsername, req.RequesterEmail,
		req.Department, req.DatasetType, req.UseCase, req.Status, req.Priority,
		req.EstimatedSize, req.Format, req.DueDate, req.Notes, req.Tags, req.ID,
	)
	return err
}

func (r *RequestStore) UpdateStatus(id int, status Status) error {
	_, err := r.db.Exec("UPDATE dataset_requests SET status=? WHERE id=?", status, id)
	return err
}

func (r *RequestStore) UpdateApproval(id int, track, decision string) error {
	col := "physics_approval"
	if track == "resources" {
		col = "resources_approval"
	}
	_, err := r.db.Exec("UPDATE dataset_requests SET "+col+"=? WHERE id=?", decision, id)
	return err
}

func (r *RequestStore) ResetApprovals(id int) error {
	_, err := r.db.Exec("UPDATE dataset_requests SET physics_approval='', resources_approval='' WHERE id=?", id)
	return err
}

func (r *RequestStore) UpdatePriority(id int, priority Priority) error {
	_, err := r.db.Exec("UPDATE dataset_requests SET priority=? WHERE id=?", priority, id)
	return err
}

func (r *RequestStore) Assign(id, assignedTo int) error {
	var uid interface{}
	if assignedTo != 0 {
		uid = assignedTo
	}
	_, err := r.db.Exec("UPDATE dataset_requests SET assigned_to=? WHERE id=?", uid, id)
	return err
}

func (r *RequestStore) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM dataset_requests WHERE id=?", id)
	return err
}

func (r *RequestStore) GetStats() (*Stats, error) {
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

func (r *RequestStore) GetRecent(limit int) ([]*DatasetRequest, error) {
	rows, err := r.db.Query(`SELECT`+selectCols+`
		FROM dataset_requests dr
		LEFT JOIN users au ON au.id = dr.assigned_to
		ORDER BY dr.created_at DESC LIMIT ?`, limit)
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

var timeFormats = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05+00:00",
	"2006-01-02T15:04:05+00:00",
}

func parseTime(s string) time.Time {
	for _, f := range timeFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanRequest(row scannable) (*DatasetRequest, error) {
	var req DatasetRequest
	var createdAt, updatedAt string
	err := row.Scan(
		&req.ID, &req.Title, &req.Description, &req.RequesterName, &req.RequesterUsername, &req.RequesterEmail,
		&req.Department, &req.DatasetType, &req.UseCase, &req.Status, &req.Priority,
		&req.EstimatedSize, &req.Format, &req.DueDate, &req.Notes, &req.Tags,
		&req.CreatedBy, &req.AssignedTo, &req.AssignedToName,
		&req.PhysicsApproval, &req.ResourcesApproval,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	req.CreatedAt = parseTime(createdAt)
	req.UpdatedAt = parseTime(updatedAt)
	return &req, nil
}
