package models

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var refPattern = regexp.MustCompile(`#(\d+)`)

type RelationType string

const (
	RelationExtends   RelationType = "extends"
	RelationDependsOn RelationType = "depends_on"
	RelationVariant   RelationType = "variant"
	RelationRelated   RelationType = "related"
	RelationMentions  RelationType = "mentions"
)

// RelationTypeLabels are the types available in the manual form (not mentions — that's auto-created).
var RelationTypeLabels = []Option{
	{"extends", "Extends"},
	{"depends_on", "Depends on"},
	{"variant", "Variant of"},
	{"related", "Related to"},
}

type Relation struct {
	ID            int
	FromID        int
	ToID          int
	Type          RelationType
	CreatedBy     int
	CreatedAt     time.Time
	RelatedID     int
	RelatedTitle  string
	RelatedStatus Status
	IsFrom        bool
}

func (rel *Relation) Label() string {
	switch rel.Type {
	case RelationExtends:
		if rel.IsFrom {
			return "Extends"
		}
		return "Extended by"
	case RelationDependsOn:
		if rel.IsFrom {
			return "Depends on"
		}
		return "Dependency of"
	case RelationVariant:
		return "Variant of"
	case RelationRelated:
		return "Related to"
	case RelationMentions:
		if rel.IsFrom {
			return "Mentions"
		}
		return "Mentioned in"
	}
	return string(rel.Type)
}

func (rel *Relation) RelatedStatusLabel() string {
	return statusLabel(string(rel.RelatedStatus))
}

func statusLabel(s string) string {
	switch s {
	case "draft":
		return "Draft"
	case "pending":
		return "Under Review"
	case "approved":
		return "Approved"
	case "in_progress":
		return "In Progress"
	case "completed":
		return "Completed"
	case "rejected":
		return "Rejected"
	case "cancelled":
		return "Cancelled"
	}
	return s
}

type RelationStore struct {
	db *sql.DB
	dbHelper
}

func NewRelationStore(db *sql.DB, driver string) *RelationStore {
	return &RelationStore{db: db, dbHelper: newHelper(driver)}
}

// CreateMentions parses #N references from texts and inserts mention relations.
// Duplicates, self-references, and references to non-existent requests are silently ignored.
func (rs *RelationStore) CreateMentions(fromID, createdBy int, texts ...string) {
	seen := map[int]bool{}
	for _, text := range texts {
		for _, match := range refPattern.FindAllStringSubmatch(text, -1) {
			toID, err := strconv.Atoi(match[1])
			if err != nil || toID == fromID || seen[toID] {
				continue
			}
			seen[toID] = true
			rs.Add(fromID, toID, createdBy, RelationMentions) //nolint:errcheck
		}
	}
}

func (rs *RelationStore) Add(fromID, toID, createdBy int, relType RelationType) error {
	_, err := rs.db.Exec(
		rs.rebind(`INSERT INTO request_relations (from_id, to_id, type, created_by) VALUES (?, ?, ?, ?)`),
		fromID, toID, relType, createdBy,
	)
	return err
}

func (rs *RelationStore) Remove(id int) error {
	_, err := rs.db.Exec(rs.rebind(`DELETE FROM request_relations WHERE id = ?`), id)
	return err
}

func (rs *RelationStore) GetByID(id int) (*Relation, error) {
	var rel Relation
	err := rs.db.QueryRow(
		rs.rebind(`SELECT id, from_id, to_id, type, COALESCE(created_by, 0) FROM request_relations WHERE id = ?`),
		id,
	).Scan(&rel.ID, &rel.FromID, &rel.ToID, &rel.Type, &rel.CreatedBy)
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

func (rs *RelationStore) GetByRequestID(requestID int) ([]*Relation, error) {
	q := rs.rebind(`
		SELECT r.id, r.from_id, r.to_id, r.type, COALESCE(r.created_by, 0), r.created_at,
		       CASE WHEN r.from_id = ? THEN r.to_id ELSE r.from_id END,
		       dr.title, dr.status
		FROM request_relations r
		JOIN dataset_requests dr ON dr.id = CASE WHEN r.from_id = ? THEN r.to_id ELSE r.from_id END
		WHERE r.from_id = ? OR r.to_id = ?
		ORDER BY r.created_at ASC`)
	rows, err := rs.db.Query(q, requestID, requestID, requestID, requestID)
	if err != nil {
		return nil, fmt.Errorf("query relations: %w", err)
	}
	defer rows.Close()

	var relations []*Relation
	for rows.Next() {
		var rel Relation
		if err := rows.Scan(
			&rel.ID, &rel.FromID, &rel.ToID, &rel.Type, &rel.CreatedBy,
			timeVal{&rel.CreatedAt},
			&rel.RelatedID, &rel.RelatedTitle, &rel.RelatedStatus,
		); err != nil {
			return nil, err
		}
		rel.IsFrom = rel.FromID == requestID
		relations = append(relations, &rel)
	}
	return relations, rows.Err()
}
