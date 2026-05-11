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

func New(db *sql.DB, driver string, oidcClient *auth.Client, devMode bool) *Handler {
	h := &Handler{
		requests:  models.NewRequestStore(db, driver),
		users:     models.NewUserStore(db, driver),
		updates:   models.NewUpdateStore(db, driver),
		oidc:      oidcClient,
		devMode:   devMode,
		emailCfg:  email.ConfigFromEnv(),
	}
	h.funcMap = template.FuncMap{
		"useCaseLabels":    func() []models.Option { return models.UseCaseLabels },
		"datasetTypeLabels": func() []models.Option { return models.DatasetTypeLabels },
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
		"nextSortDir": func(col, currentSort, currentDir string) string {
			if col != currentSort {
				switch col {
				case "priority", "created", "updated":
					return "desc"
				default:
					return "asc"
				}
			}
			if currentDir == "asc" {
				return "desc"
			}
			return "asc"
		},
		"sortIcon": func(col, currentSort, currentDir string) template.HTML {
			if col != currentSort {
				return template.HTML(`<svg class="w-3 h-3 text-stone-300 dark:text-stone-600 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M8 9l4-4 4 4m0 6l-4 4-4-4"/></svg>`)
			}
			if currentDir == "asc" {
				return template.HTML(`<svg class="w-3 h-3 text-indigo-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M4.5 15.75l7.5-7.5 7.5 7.5"/></svg>`)
			}
			return template.HTML(`<svg class="w-3 h-3 text-indigo-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5"/></svg>`)
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
	Pagination  *Pagination
	CurrentUser *models.User
	Error       string
	DevMode     bool
	Updates     []*models.Update
	Managers    []*models.User
	IsPage      bool // true when rendered as a standalone page, not a modal fragment
}

type FilterState struct {
	Status   string
	Priority string
	Search   string
	Sort     string
	SortDir  string
	Page     int
	PerPage  int
}

type Pagination struct {
	Page       int
	PerPage    int
	Total      int
	TotalPages int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
	From       int
	To         int
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
	subject := fmt.Sprintf("[FCC-DRS] Request #%d status updated: %s", req.ID, newStatus)
	body := fmt.Sprintf(
		"Your dataset request \"%s\" (ID: %d) has been updated.\n\nNew status: %s\n\nFCC Dataset Request System",
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
	q := r.URL.Query()
	status := q.Get("status")
	priority := q.Get("priority")
	search := q.Get("search")
	sort := q.Get("sort")
	sortDir := q.Get("dir")

	validSorts := map[string]bool{"title": true, "requester": true, "status": true, "priority": true, "created": true, "updated": true}
	if !validSorts[sort] {
		sort = ""
	}
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "desc"
	}

	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if page <= 0 {
		page = 1
	}
	if perPage != 10 && perPage != 20 && perPage != 50 {
		perPage = 20
	}

	requests, total, err := h.requests.GetAll(status, priority, search, sort, sortDir, page, perPage)
	if err != nil {
		slog.Error("list requests", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	from := (page-1)*perPage + 1
	to := page * perPage
	if to > total {
		to = total
	}
	if total == 0 {
		from = 0
	}
	pagination := &Pagination{
		Page: page, PerPage: perPage, Total: total, TotalPages: totalPages,
		HasPrev: page > 1, HasNext: page < totalPages,
		PrevPage: page - 1, NextPage: page + 1,
		From: from, To: to,
	}

	filter := FilterState{Status: status, Priority: priority, Search: search, Sort: sort, SortDir: sortDir, Page: page, PerPage: perPage}

	if r.Header.Get("HX-Request") == "true" {
		h.renderPartial(w, r, "request_list", PageData{Requests: requests, Filter: filter, Pagination: pagination})
		return
	}

	stats, _ := h.requests.GetStats()
	h.renderPage(w, r, "requests", PageData{
		Title:      "All Requests",
		Requests:   requests,
		Stats:      stats,
		Filter:     filter,
		Pagination: pagination,
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
		Statistics:        strings.TrimSpace(r.FormValue("statistics")),
		TargetCampaign:    strings.TrimSpace(r.FormValue("target_campaign")),
		Key4hepStack:      strings.TrimSpace(r.FormValue("key4hep_stack")),
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
	if req.Status != models.StatusDraft && (req.UseCase == "" || req.DatasetType == "" || req.Department == "") {
		http.Error(w, "use case, dataset stage, and group/team are required", 400)
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

	activity, _ := h.updates.GetByRequestID(id)
	managers, _ := h.users.GetManagers()

	if r.Header.Get("HX-Request") == "true" {
		h.renderPartial(w, r, "request_detail", PageData{
			Request: req, Updates: activity, Managers: managers,
		})
		return
	}
	h.renderPage(w, r, "request_detail_page", PageData{
		Title: req.Title, Request: req, Updates: activity, Managers: managers, IsPage: true,
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
		Status:            existing.Status,
		Priority:          models.Priority(r.FormValue("priority")),
		EstimatedSize:     strings.TrimSpace(r.FormValue("estimated_size")),
		Statistics:        strings.TrimSpace(r.FormValue("statistics")),
		TargetCampaign:    strings.TrimSpace(r.FormValue("target_campaign")),
		Key4hepStack:      strings.TrimSpace(r.FormValue("key4hep_stack")),
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

	user := middleware.GetUser(r)
	status := models.Status(r.FormValue("status"))

	if !user.IsManager() {
		if existing.CreatedBy != user.ID {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if status != models.StatusDraft && status != models.StatusCancelled {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	if err := h.requests.UpdateStatus(id, status); err != nil {
		slog.Error("update status", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

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

	htmxTarget := r.Header.Get("HX-Target")
	if strings.HasPrefix(htmxTarget, "status-cell-") {
		h.renderPartial(w, r, "status_badge", PageData{Request: req})
		return
	}
	updates, _ := h.updates.GetByRequestID(id)
	managers, _ := h.users.GetManagers()
	h.renderPartial(w, r, "request_detail", PageData{
		Request:  req,
		Updates:  updates,
		Managers: managers,
		IsPage:   htmxTarget != "modal-container",
	})
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

func (h *Handler) ApprovalDecision(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	track := r.FormValue("track")       // "physics" or "resources"
	decision := r.FormValue("decision") // "approved" or "rejected"

	if track != "physics" && track != "resources" {
		http.Error(w, "invalid track", 400)
		return
	}
	if decision != "approved" && decision != "rejected" && decision != "revert" {
		http.Error(w, "invalid decision", 400)
		return
	}

	existing, err := h.requests.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}

	user := middleware.GetUser(r)
	userID := 0
	userName := ""
	if user != nil {
		userID = user.ID
		userName = user.DisplayName
	}

	trackLabel := "Physics"
	if track == "resources" {
		trackLabel = "Resources"
	}

	approvalValue := decision
	if decision == "revert" {
		approvalValue = ""
	}

	if err := h.requests.UpdateApproval(id, track, approvalValue); err != nil {
		slog.Error("update approval", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	body := trackLabel + " approval: " + decision
	if userName != "" {
		body += " (by " + userName + ")"
	}
	h.updates.Add(id, userID, models.UpdateStatusChanged, body)

	// Reload to get updated approval fields.
	req, err := h.requests.GetByID(id)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	// Auto-promote when both tracks approved.
	if req.PhysicsApproval == "approved" && req.ResourcesApproval == "approved" && req.Status == models.StatusPending {
		if err := h.requests.UpdateStatus(id, models.StatusApproved); err == nil {
			h.updates.Add(id, userID, models.UpdateStatusChanged, "under review → approved (both approvals granted)")
			h.sendStatusEmail(existing, models.StatusApproved)
		}
	} else if decision == "rejected" && req.Status == models.StatusPending {
		if err := h.requests.UpdateStatus(id, models.StatusRejected); err == nil {
			h.updates.Add(id, userID, models.UpdateStatusChanged, "under review → rejected ("+trackLabel+" approval denied)")
			h.sendStatusEmail(existing, models.StatusRejected)
		}
	} else if decision == "revert" && (req.Status == models.StatusApproved || req.Status == models.StatusRejected || req.Status == models.StatusCompleted || req.Status == models.StatusInProgress) {
		// Revert overall status back to under review when an approval is revoked.
		if err := h.requests.UpdateStatus(id, models.StatusPending); err == nil {
			h.updates.Add(id, userID, models.UpdateStatusChanged, string(req.Status)+" → under review ("+trackLabel+" approval reverted)")
		}
	}

	// Reload again after potential status change.
	req, _ = h.requests.GetByID(id)
	updates, _ := h.updates.GetByRequestID(id)
	managers, _ := h.users.GetManagers()
	h.renderPartial(w, r, "request_detail", PageData{Request: req, Updates: updates, Managers: managers})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.requests.GetStats()
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	h.renderPartial(w, r, "stats_cards", PageData{Stats: stats})
}

func (h *Handler) GetRecent(w http.ResponseWriter, r *http.Request) {
	recent, err := h.requests.GetRecent(6)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	h.renderPartial(w, r, "recent_requests", PageData{Recent: recent})
}
