package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"syntopica-backend/internal/dataenrichment/repository"
)

// ── Reference roles (methodology profiles; design D5) ───────────────────────
//
// Roles shape HOW the agent analyzes (method, never facts — spec red line).
// Enable/disable takes effect on the next orchestration because the injection
// reads ListEnabledReferenceRoles fresh every run (no cache).

type referenceRoleRequest struct {
	Name    *string `json:"name"`
	Title   *string `json:"title"`
	Content *string `json:"content"`
	Enabled *bool   `json:"enabled"`
}

// listReferenceRoles returns all roles (admin listing).
// GET /reference-roles
func (h *EnrichmentHandler) listReferenceRoles(c *gin.Context) {
	list, err := h.repo.ListReferenceRoles(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, list)
}

// createReferenceRole inserts a role. Enabled defaults to true when omitted.
// POST /reference-roles
func (h *EnrichmentHandler) createReferenceRole(c *gin.Context) {
	var req referenceRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	name := deref(req.Name)
	content := deref(req.Content)
	if name == "" || content == "" {
		respondError(c, http.StatusBadRequest, "name and content are required")
		return
	}
	role := &repository.ReferenceRole{
		Name:    name,
		Title:   deref(req.Title),
		Content: content,
		Enabled: req.Enabled == nil || *req.Enabled,
	}
	if err := h.repo.CreateReferenceRole(c.Request.Context(), role); err != nil {
		if isUniqueViolation(err) {
			respondError(c, http.StatusConflict, "reference role name already exists")
			return
		}
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, role)
}

// getReferenceRole fetches one role.
// GET /reference-roles/:id
func (h *EnrichmentHandler) getReferenceRole(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	role, err := h.repo.GetReferenceRoleByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}
	respondOK(c, role)
}

// updateReferenceRole applies partial updates ({name?, title?, content?, enabled?}).
// PUT /reference-roles/:id — enable/disable rides on this endpoint (settings UI
// toggles send {"enabled": false}; no separate toggle route).
func (h *EnrichmentHandler) updateReferenceRole(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req referenceRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	role, err := h.repo.GetReferenceRoleByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			respondError(c, http.StatusBadRequest, "name cannot be empty")
			return
		}
		role.Name = strings.TrimSpace(*req.Name)
	}
	if req.Title != nil {
		role.Title = *req.Title
	}
	if req.Content != nil {
		if strings.TrimSpace(*req.Content) == "" {
			respondError(c, http.StatusBadRequest, "content cannot be empty")
			return
		}
		role.Content = *req.Content
	}
	if req.Enabled != nil {
		role.Enabled = *req.Enabled
	}
	if err := h.repo.UpdateReferenceRole(c.Request.Context(), role); err != nil {
		if isUniqueViolation(err) {
			respondError(c, http.StatusConflict, "reference role name already exists")
			return
		}
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, role)
}

// deleteReferenceRole removes a role permanently.
// DELETE /reference-roles/:id
func (h *EnrichmentHandler) deleteReferenceRole(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.repo.DeleteReferenceRole(c.Request.Context(), id); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, gin.H{"deleted": id})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

// isUniqueViolation matches both Postgres (23505 unique_violation) and the
// SQLite test driver (UNIQUE constraint failed) so repo tests share the path.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
