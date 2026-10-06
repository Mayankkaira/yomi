package audit

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yomi/yomi/internal/models"
)

// Handler exposes the Audit Ledger HTTP API.
type Handler struct {
	repo *Repository
}

// NewHandler wires the repository into an HTTP handler.
func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// ByCitizen returns ledger entries for a citizen.
func (h *Handler) ByCitizen(c *gin.Context) {
	entries, err := h.repo.ByCitizen(c.Request.Context(), c.Param("citizenId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit trail", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

// ByInstitution returns ledger entries for an institution.
func (h *Handler) ByInstitution(c *gin.Context) {
	entries, err := h.repo.ByInstitution(c.Request.Context(), c.Param("institutionId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit trail", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

// ByID returns a single ledger entry by event ID.
func (h *Handler) ByID(c *gin.Context) {
	entry, err := h.repo.ByID(c.Request.Context(), c.Param("eventId"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "audit entry not found", "code": "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit entry", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entry})
}

// Verify returns the integrity status of the hash chain.
func (h *Handler) Verify(c *gin.Context) {
	links, err := h.repo.VerifyChain(c.Request.Context(), 1000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify ledger", "code": "INTERNAL"})
		return
	}
	valid := true
	for _, l := range links {
		if !l.Valid {
			valid = false
			break
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"intact": valid, "entriesChecked": len(links), "chain": links}})
}

var _ = models.AuditEntry{}
