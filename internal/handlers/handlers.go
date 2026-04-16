package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dataset-tracker/internal/auth"
	"dataset-tracker/internal/email"
	"dataset-tracker/internal/middleware"
	"dataset-tracker/internal/models"
)

type Handler struct {
	requests  *models.RequestStore
	users     *models.UserStore
	updates    *models.UpdateStore
	oidc      *auth.Client
	funcMap   template.FuncMap
	devMode   bool
	emailCfg  email.Config
}

func New(db *sql.DB, oidcClient *auth.Client, devMode bool) *Handler {
	h := &Handler{
		requests:  models.NewRequestStore(db),
		users:     models.NewUserStore(db),
		updates:   models.NewUpdateStore(db),
		oidc:      oidcClient,
		devMode:   devMode,
		emailCfg:  email.ConfigFromEnv(),
	}
	h.funcMap = template.FuncMap{
		"statusClass":    statusClass,
		"priorityClass":  priorityClass,
		"truncate":       truncate,
		"formatDate":     formatDate,
		"formatDateTime": formatDateTime,
		"timeAgo":        timeAgo,
		"add":            func(a, b int) int { return a + b },
		"currentYear":    func() int { return time.Now().Year() },
		"initial": func(s string) string {
			runes := []rune(s)
			if len(runes) == 0 {
				return "?"
			}
			return string(runes[0])
		},
	}
	return h
}

// renderPage parses layout + a specific page template together.
// CurrentUser is automatically populated from the request context.
func (h *Handler) renderPage(w http.ResponseWriter, r *http.Request, page string, data PageData) {
	data.CurrentUser = middleware.GetUser(r)
	tmpl, err := template.New("").Funcs(h.funcMap).ParseFiles(
		"templates/layout.html",
		"templates/"+page+".html",
	)
	if err != nil {
		slog.Error("parse page template", "page", page, "error", err)
		http.Error(w, "template error: "+err.Error(), 500)
		return
	}
	if _, err2 := tmpl.ParseGlob("templates/partials/*.html"); err2 != nil {
		slog.Error("parse partials", "error", err2)
	}
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("execute page template", "page", page, "error", err)
	}
}

// renderPartial renders a named partial template for HTMX responses.
func (h *Handler) renderPartial(w http.ResponseWriter, r *http.Request, name string, data PageData) {
	data.CurrentUser = middleware.GetUser(r)
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

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "yesterday"
	}
	if days < 7 {
		return fmt.Sprintf("%d days ago", days)
	}
	return t.Format("Jan 2, 2006")
}

// ── page data ─────────────────────────────────────────────────────────────────

type PageData struct {
	Title       string
	Requests    []*models.DatasetRequest
	Request     *models.DatasetRequest
	Stats       *models.Stats
	Recent      []*models.DatasetRequest
	Filter      FilterState
	CurrentUser *models.User
	Error       string
	DevMode     bool
	Updates     []*models.Update
	Managers    []*models.User

}

type FilterState struct {
	Status   string
	Priority string
	Search   string
}

// canEdit returns true when the current user may edit the given request.
func canEdit(user *models.User, req *models.DatasetRequest) bool {
	if user == nil {
		return false
	}
	if user.IsManager() {
		return true
	}
	if req.CreatedBy != user.ID {
		return false
	}
	return req.Status == models.StatusDraft || req.Status == models.StatusPending
}

// sendStatusEmail notifies the requester of a status change (no-op if email unconfigured).
func (h *Handler) sendStatusEmail(req *models.DatasetRequest, newStatus models.Status) {
	if !h.emailCfg.Enabled() || req.RequesterEmail == "" {
		return
	}
	subject := fmt.Sprintf("[Bob the Tracker] Request #%d status updated: %s", req.ID, newStatus)
	body := fmt.Sprintf(
		"Your dataset request \"%s\" (ID: %d) has been updated.\n\nNew status: %s\n\nBob the Tracker — FCC Dataset Request System",
		req.Title, req.ID, req.StatusLabel(),
	)
	if err := h.emailCfg.Send(req.RequesterEmail, subject, body); err != nil {
		slog.Error("send status email", "request_id", req.ID, "error", err)
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.requests.GetStats()
	if err != nil {
		slog.Error("get stats", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	recent, err := h.requests.GetRecent(6)
	if err != nil {
		slog.Error("get recent", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	h.renderPage(w, r, "index", PageData{
		Title:  "Dashboard",
		Stats:  stats,
		Recent: recent,
	})
}

func (h *Handler) ListRequests(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	priority := r.URL.Query().Get("priority")
	search := r.URL.Query().Get("search")

	requests, err := h.requests.GetAll(status, priority, search)
	if err != nil {
		slog.Error("list requests", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	filter := FilterState{Status: status, Priority: priority, Search: search}

	if r.Header.Get("HX-Request") == "true" {
		h.renderPartial(w, r, "request_list", PageData{Requests: requests, Filter: filter})
		return
	}

	stats, _ := h.requests.GetStats()
	h.renderPage(w, r, "requests", PageData{
		Title:    "All Requests",
		Requests: requests,
		Stats:    stats,
		Filter:   filter,
	})
}

func (h *Handler) NewRequestForm(w http.ResponseWriter, r *http.Request) {
	h.renderPartial(w, r, "request_form", PageData{Title: "New Request"})
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

	user := middleware.GetUser(r)
	createdBy := 0
	if user != nil {
		createdBy = user.ID
	}

	req := &models.DatasetRequest{
		Title:             strings.TrimSpace(r.FormValue("title")),
		Description:       strings.TrimSpace(r.FormValue("description")),
		RequesterName:     func() string { if user != nil { return user.DisplayName }; return "" }(),
		RequesterUsername: func() string { if user != nil { return user.Username }; return "" }(),
		RequesterEmail:    func() string { if user != nil { return user.Email }; return "" }(),
		Department:        strings.TrimSpace(r.FormValue("department")),
		DatasetType:       r.FormValue("dataset_type"),
		UseCase:           r.FormValue("use_case"),
		Status:            status,
		Priority:          priority,
		EstimatedSize:     strings.TrimSpace(r.FormValue("estimated_size")),
		Format:            strings.TrimSpace(r.FormValue("format")),
		DueDate:           r.FormValue("due_date"),
		Notes:             strings.TrimSpace(r.FormValue("notes")),
		Tags:              strings.TrimSpace(r.FormValue("tags")),
		CreatedBy:         createdBy,
	}

	if req.Title == "" {
		http.Error(w, "title is required", 400)
		return
	}

	id, err := h.requests.Create(req)
	if err != nil {
		slog.Error("create request", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	slog.Info("created request", "id", id)

	// Log creation event
	userName := "unknown"
	if user != nil {
		userName = user.DisplayName
	}
	h.updates.Add(int(id), createdBy, models.UpdateCreated, "Request submitted by "+userName)

	w.Header().Set("HX-Redirect", "/requests")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	req, err := h.requests.GetByID(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Not Found", 404)
		return
	}
	if err != nil {
		slog.Error("get request", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	events, _ := h.updates.GetByRequestID(id)
	managers, _ := h.users.GetManagers()

	if r.Header.Get("HX-Request") == "true" {
		h.renderPartial(w, r, "request_detail", PageData{
			Request: req, Updates: events, Managers: managers,
		})
		return
	}
	h.renderPage(w, r, "request_detail_page", PageData{
		Title: req.Title, Request: req, Updates: events, Managers: managers,
	})
}

func (h *Handler) EditRequestForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	req, err := h.requests.GetByID(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Not Found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	user := middleware.GetUser(r)
	if !canEdit(user, req) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	h.renderPartial(w, r, "request_form", PageData{Title: "Edit Request", Request: req})
}

func (h *Handler) UpdateRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}

	existing, err := h.requests.GetByID(id)
	if err == sql.ErrNoRows {
		http.Error(w, "Not Found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	user := middleware.GetUser(r)
	if !canEdit(user, existing) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	req := &models.DatasetRequest{
		ID:                id,
		Title:             strings.TrimSpace(r.FormValue("title")),
		Description:       strings.TrimSpace(r.FormValue("description")),
		RequesterName:     existing.RequesterName,
		RequesterUsername: existing.RequesterUsername,
		RequesterEmail:    existing.RequesterEmail,
		Department:        strings.TrimSpace(r.FormValue("department")),
		DatasetType:       r.FormValue("dataset_type"),
		UseCase:           r.FormValue("use_case"),
		Status:            models.Status(r.FormValue("status")),
		Priority:          models.Priority(r.FormValue("priority")),
		EstimatedSize:     strings.TrimSpace(r.FormValue("estimated_size")),
		Format:            strings.TrimSpace(r.FormValue("format")),
		DueDate:           r.FormValue("due_date"),
		Notes:             strings.TrimSpace(r.FormValue("notes")),
		Tags:              strings.TrimSpace(r.FormValue("tags")),
	}

	if err := h.requests.Update(req); err != nil {
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

	existing, err := h.requests.GetByID(id)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	status := models.Status(r.FormValue("status"))
	if err := h.requests.UpdateStatus(id, status); err != nil {
		slog.Error("update status", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	user := middleware.GetUser(r)
	userID := 0
	userName := ""
	if user != nil {
		userID = user.ID
		userName = user.DisplayName
	}
	body := string(existing.Status) + " → " + string(status)
	if userName != "" {
		body += " (by " + userName + ")"
	}
	h.updates.Add(id, userID, models.UpdateStatusChanged, body)
	h.sendStatusEmail(existing, status)

	req, err := h.requests.GetByID(id)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	h.renderPartial(w, r, "status_badge", PageData{Request: req})
}

func (h *Handler) DeleteRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	if err := h.requests.Delete(id); err != nil {
		slog.Error("delete request", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	w.Header().Set("HX-Redirect", "/requests")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.requests.GetStats()
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	h.renderPartial(w, r, "stats_cards", PageData{Stats: stats})
}
