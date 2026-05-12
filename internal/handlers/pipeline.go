package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"dataset-tracker/internal/middleware"
	"dataset-tracker/internal/models"
)

func (h *Handler) ManagerView(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/requests", http.StatusMovedPermanently)
}

func (h *Handler) AddComment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		http.Error(w, "comment body required", 400)
		return
	}

	user := middleware.GetUser(r)
	userID := 0
	if user != nil {
		userID = user.ID
	}

	evType := models.UpdateComment
	if user.IsManager() && r.FormValue("internal") == "1" {
		evType = models.UpdateInternalNote
	}

	if err := h.updates.Add(id, userID, evType, body); err != nil {
		slog.Error("add comment", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	h.relations.CreateMentions(id, userID, body)

	req, _ := h.requests.GetByID(id)
	stages, _ := h.updates.GetByRequestID(id)
	h.renderPartial(w, r, "activity", PageData{Request: req, Updates: stages})
}

func (h *Handler) GetComment(w http.ResponseWriter, r *http.Request) {
	commentID, err := strconv.Atoi(r.PathValue("comment_id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	comment, err := h.updates.GetByID(commentID)
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	h.renderPartial(w, r, "comment_view", PageData{Comment: comment})
}

func (h *Handler) EditCommentForm(w http.ResponseWriter, r *http.Request) {
	commentID, err := strconv.Atoi(r.PathValue("comment_id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	comment, err := h.updates.GetByID(commentID)
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	user := middleware.GetUser(r)
	if user == nil || (!user.IsManager() && comment.UserID != user.ID) {
		http.Error(w, "Forbidden", 403)
		return
	}
	h.renderPartial(w, r, "comment_edit", PageData{Comment: comment})
}

func (h *Handler) PatchComment(w http.ResponseWriter, r *http.Request) {
	commentID, err := strconv.Atoi(r.PathValue("comment_id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	comment, err := h.updates.GetByID(commentID)
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	user := middleware.GetUser(r)
	if user == nil || (!user.IsManager() && comment.UserID != user.ID) {
		http.Error(w, "Forbidden", 403)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		http.Error(w, "comment body required", 400)
		return
	}
	if err := h.updates.UpdateBody(commentID, body); err != nil {
		slog.Error("patch comment", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	comment.Body = body
	h.renderPartial(w, r, "comment_view", PageData{Comment: comment})
}

func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	commentID, err := strconv.Atoi(r.PathValue("comment_id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	comment, err := h.updates.GetByID(commentID)
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	user := middleware.GetUser(r)
	if user == nil || (!user.IsManager() && comment.UserID != user.ID) {
		http.Error(w, "Forbidden", 403)
		return
	}
	if err := h.updates.DeleteByID(commentID); err != nil {
		slog.Error("delete comment", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	req, _ := h.requests.GetByID(id)
	stages, _ := h.updates.GetByRequestID(id)
	h.renderPartial(w, r, "activity", PageData{Request: req, Updates: stages})
}

func (h *Handler) AssignRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	assignedTo, _ := strconv.Atoi(r.FormValue("assigned_to"))

	user := middleware.GetUser(r)
	userID := 0
	if user != nil {
		userID = user.ID
	}

	if err := h.requests.Assign(id, assignedTo); err != nil {
		slog.Error("assign request", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	var eventBody string
	if assignedTo == 0 {
		eventBody = "Unassigned"
	} else {
		if assignee, err := h.users.GetByID(assignedTo); err == nil {
			eventBody = "Assigned to " + assignee.DisplayName
		} else {
			eventBody = "Assigned"
		}
	}
	h.updates.Add(id, userID, models.UpdateAssigned, eventBody)

	req, err := h.requests.GetByID(id)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}
	managers, _ := h.users.GetManagers()
	h.renderPartial(w, r, "assignment", PageData{Request: req, Managers: managers})
}

func (h *Handler) UpdatePriority(w http.ResponseWriter, r *http.Request) {
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

	priority := models.Priority(r.FormValue("priority"))
	if err := h.requests.UpdatePriority(id, priority); err != nil {
		slog.Error("update priority", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	user := middleware.GetUser(r)
	userID := 0
	if user != nil {
		userID = user.ID
	}
	body := string(existing.Priority) + " → " + string(priority)
	h.updates.Add(id, userID, models.UpdatePriorityChanged, body)

	req, err := h.requests.GetByID(id)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	htmxTarget := r.Header.Get("HX-Target")
	if strings.HasPrefix(htmxTarget, "priority-cell-") {
		h.renderPartial(w, r, "priority_cell", PageData{Request: req})
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

func (h *Handler) BatchAction(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	action := r.FormValue("action")
	ids := r.Form["ids"]

	var status models.Status
	switch action {
	case "approve":
		status = models.StatusApproved
	case "in_progress":
		status = models.StatusInProgress
	case "complete":
		status = models.StatusCompleted
	case "reject":
		status = models.StatusRejected
	default:
		http.Error(w, "unknown action", 400)
		return
	}

	user := middleware.GetUser(r)
	userID := 0
	userName := ""
	if user != nil {
		userID = user.ID
		userName = user.DisplayName
	}

	for _, idStr := range ids {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		existing, err := h.requests.GetByID(id)
		if err != nil {
			continue
		}
		if err := h.requests.UpdateStatus(id, status); err != nil {
			slog.Error("batch update status", "id", id, "error", err)
			continue
		}
		body := string(existing.Status) + " → " + string(status)
		if userName != "" {
			body += " (by " + userName + ")"
		}
		h.updates.Add(id, userID, models.UpdateStatusChanged, body)
		h.sendStatusEmail(existing, status)
	}

	w.Header().Set("HX-Redirect", r.Header.Get("HX-Current-URL"))
	if w.Header().Get("HX-Redirect") == "" {
		w.Header().Set("HX-Redirect", "/requests")
	}
	w.WriteHeader(http.StatusOK)
}
