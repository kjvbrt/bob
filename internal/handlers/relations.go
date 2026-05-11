package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"dataset-tracker/internal/middleware"
	"dataset-tracker/internal/models"
)

func (h *Handler) AddRelation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	req, err := h.requests.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}

	cu := middleware.GetUser(r)

	if !cu.IsManager() {
		canEdit := req.CreatedBy == cu.ID && (req.Status == "draft" || req.Status == "pending")
		if !canEdit {
			http.Error(w, "Forbidden", 403)
			return
		}
	}

	toIDStr := strings.TrimPrefix(strings.TrimSpace(r.FormValue("to_id")), "#")
	toID, err := strconv.Atoi(toIDStr)
	if err != nil || toID <= 0 || toID == id {
		h.renderRelations(w, r, req, cu, "Invalid request ID.")
		return
	}

	_, err = h.requests.GetByID(toID)
	if err != nil {
		h.renderRelations(w, r, req, cu, "Request #"+strconv.Itoa(toID)+" not found.")
		return
	}

	relType := models.RelationType(r.FormValue("type"))
	switch relType {
	case models.RelationExtends, models.RelationDependsOn, models.RelationVariant, models.RelationRelated:
	default:
		relType = models.RelationRelated
	}

	if err := h.relations.Add(id, toID, cu.ID, relType); err != nil {
		h.renderRelations(w, r, req, cu, "Could not add relation (it may already exist).")
		return
	}

	h.renderRelations(w, r, req, cu, "")
}

func (h *Handler) RemoveRelation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	req, err := h.requests.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}

	cu := middleware.GetUser(r)

	if !cu.IsManager() {
		canEdit := req.CreatedBy == cu.ID && (req.Status == "draft" || req.Status == "pending")
		if !canEdit {
			http.Error(w, "Forbidden", 403)
			return
		}
	}

	relID, err := strconv.Atoi(r.PathValue("rel_id"))
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	h.relations.Remove(relID)
	h.renderRelations(w, r, req, cu, "")
}

func (h *Handler) renderRelations(w http.ResponseWriter, r *http.Request, req *models.DatasetRequest, cu *models.User, errMsg string) {
	relations, _ := h.relations.GetByRequestID(req.ID)
	h.renderPartial(w, r, "relations", PageData{
		Request: req, Relations: relations, CurrentUser: cu, Error: errMsg,
	})
}
