package handlers

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dataset-tracker/internal/models"
)

type Handler struct {
	repo     *models.Repository
	funcMap  template.FuncMap
	partials *template.Template
}

func New(db *sql.DB) *Handler {
	h := &Handler{
		repo: models.NewRepository(db),
	}
	h.funcMap = template.FuncMap{
		"statusClass":    statusClass,
		"priorityClass":  priorityClass,
		"truncate":       truncate,
		"formatDate":     formatDate,
		"formatDateTime": formatDateTime,
		"add":            func(a, b int) int { return a + b },
		"initial": func(s string) string {
			runes := []rune(s)
			if len(runes) == 0 {
				return "?"
			}
			return string(runes[0])
		},
	}
	// Pre-load partials for HTMX responses
	h.partials = template.Must(
		template.New("").Funcs(h.funcMap).ParseGlob("templates/partials/*.html"),
	)
	return h
}

// renderPage parses layout + a specific page template together so that
// {{define "content"}} in the page file overrides {{block "content"}} in layout.
func (h *Handler) renderPage(w http.ResponseWriter, page string, data any) {
	tmpl, err := template.New("").Funcs(h.funcMap).ParseFiles(
		"templates/layout.html",
		"templates/"+page+".html",
	)
	if err != nil {
		slog.Error("parse page template", "page", page, "error", err)
		http.Error(w, "template error: "+err.Error(), 500)
		return
	}
	// Also absorb partials so pages can embed them
	if _, err2 := tmpl.ParseGlob("templates/partials/*.html"); err2 != nil {
		slog.Error("parse partials", "error", err2)
	}
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("execute page template", "page", page, "error", err)
	}
}

func (h *Handler) renderPartial(w http.ResponseWriter, name string, data any) {
	// Re-parse so template changes are picked up without restart (dev-friendly)
	tmpl, err := template.New("").Funcs(h.funcMap).ParseGlob("templates/partials/*.html")
	if err != nil {
		slog.Error("parse partials", "error", err)
		http.Error(w, "template error", 500)
		return
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("execute partial", "name", name, "error", err)
		http.Error(w, "template error", 500)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func statusClass(s models.Status) string {
	switch s {
	case models.StatusDraft:
		return "status-draft"
	case models.StatusPending:
		return "status-pending"
	case models.StatusApproved:
		return "status-approved"
	case models.StatusRejected:
		return "status-rejected"
	case models.StatusInProgress:
		return "status-inprogress"
	case models.StatusCompleted:
		return "status-completed"
	case models.StatusCancelled:
		return "status-cancelled"
	default:
		return "status-draft"
	}
}

func priorityClass(p models.Priority) string {
	switch p {
	case models.PriorityLow:
		return "priority-low"
	case models.PriorityMedium:
		return "priority-medium"
	case models.PriorityHigh:
		return "priority-high"
	case models.PriorityCritical:
		return "priority-critical"
	default:
		return "priority-low"
	}
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 2, 2006")
}

func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 2, 2006 15:04")
}

// ── page data ─────────────────────────────────────────────────────────────────

type PageData struct {
	Title    string
	Requests []*models.DatasetRequest
	Request  *models.DatasetRequest
	Stats    *models.Stats
	Recent   []*models.DatasetRequest
	Filter   FilterState
}

type FilterState struct {
	Status   string
	Priority string
	Search   string
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.repo.GetStats()
	if err != nil {
		slog.Error("get stats", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	recent, err := h.repo.GetRecent(6)
	if err != nil {
		slog.Error("get recent", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	h.renderPage(w, "index", PageData{
		Title:  "Dashboard",
		Stats:  stats,
		Recent: recent,
	})
}

func (h *Handler) ListRequests(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	priority := r.URL.Query().Get("priority")
	search := r.URL.Query().Get("search")

	requests, err := h.repo.GetAll(status, priority, search)
	if err != nil {
		slog.Error("list requests", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	filter := FilterState{Status: status, Priority: priority, Search: search}

	if r.Header.Get("HX-Request") == "true" {
		h.renderPartial(w, "request_list", PageData{Requests: requests, Filter: filter})
		return
	}

	stats, _ := h.repo.GetStats()
	h.renderPage(w, "requests", PageData{
		Title:    "All Requests",
		Requests: requests,
		Stats:    stats,
		Filter:   filter,
	})
}

func (h *Handler) NewRequestForm(w http.ResponseWriter, r *http.Request) {
	h.renderPartial(w, "request_form", PageData{Title: "New Request"})
}

func (h *Handler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	status := models.Status(r.FormValue("status"))
	if status == "" {
		status = models.StatusPending
	}
	priority := models.Priority(r.FormValue("priority"))
	if priority == "" {
		priority = models.PriorityMedium
	}

	req := &models.DatasetRequest{
		Title:          strings.TrimSpace(r.FormValue("title")),
		Description:    strings.TrimSpace(r.FormValue("description")),
		RequesterName:  strings.TrimSpace(r.FormValue("requester_name")),
		RequesterEmail: strings.TrimSpace(r.FormValue("requester_email")),
		Department:     strings.TrimSpace(r.FormValue("department")),
		DatasetType:    r.FormValue("dataset_type"),
		UseCase:        r.FormValue("use_case"),
		Status:         status,
		Priority:       priority,
		EstimatedSize:  strings.TrimSpace(r.FormValue("estimated_size")),
		Format:         strings.TrimSpace(r.FormValue("format")),
		DueDate:        r.FormValue("due_date"),
		Notes:          strings.TrimSpace(r.FormValue("notes")),
		Tags:           strings.TrimSpace(r.FormValue("tags")),
	}

	if req.Title == "" || req.RequesterName == "" {
		http.Error(w, "title and requester_name are required", 400)
		return
	}

	id, err := h.repo.Create(req)
	if err != nil {
		slog.Error("create request", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	slog.Info("created request", "id", id)

	w.Header().Set("HX-Redirect", "/requests")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	req, err := h.repo.GetByID(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Not Found", 404)
		return
	}
	if err != nil {
		slog.Error("get request", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		h.renderPartial(w, "request_detail", PageData{Request: req})
		return
	}
	h.renderPage(w, "request_detail_page", PageData{Title: req.Title, Request: req})
}

func (h *Handler) EditRequestForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	req, err := h.repo.GetByID(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Not Found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	h.renderPartial(w, "request_form", PageData{Title: "Edit Request", Request: req})
}

func (h *Handler) UpdateRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	req := &models.DatasetRequest{
		ID:             id,
		Title:          strings.TrimSpace(r.FormValue("title")),
		Description:    strings.TrimSpace(r.FormValue("description")),
		RequesterName:  strings.TrimSpace(r.FormValue("requester_name")),
		RequesterEmail: strings.TrimSpace(r.FormValue("requester_email")),
		Department:     strings.TrimSpace(r.FormValue("department")),
		DatasetType:    r.FormValue("dataset_type"),
		UseCase:        r.FormValue("use_case"),
		Status:         models.Status(r.FormValue("status")),
		Priority:       models.Priority(r.FormValue("priority")),
		EstimatedSize:  strings.TrimSpace(r.FormValue("estimated_size")),
		Format:         strings.TrimSpace(r.FormValue("format")),
		DueDate:        r.FormValue("due_date"),
		Notes:          strings.TrimSpace(r.FormValue("notes")),
		Tags:           strings.TrimSpace(r.FormValue("tags")),
	}

	if err := h.repo.Update(req); err != nil {
		slog.Error("update request", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	w.Header().Set("HX-Redirect", "/requests/"+strconv.Itoa(id))
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	status := models.Status(r.FormValue("status"))
	if err := h.repo.UpdateStatus(id, status); err != nil {
		slog.Error("update status", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	req, err := h.repo.GetByID(id)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	h.renderPartial(w, "status_badge", req)
}

func (h *Handler) DeleteRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	if err := h.repo.Delete(id); err != nil {
		slog.Error("delete request", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	w.Header().Set("HX-Redirect", "/requests")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.repo.GetStats()
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	h.renderPartial(w, "stats_cards", PageData{Stats: stats})
}
