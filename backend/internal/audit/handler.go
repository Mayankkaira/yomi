package audit

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yomi/yomi/internal/middleware"
	"github.com/yomi/yomi/internal/models"
)

type Handler struct {
	repo          *Repository
	receiptSecret []byte
}

func NewHandler(repo *Repository, receiptSecret string) *Handler {
	return &Handler{repo: repo, receiptSecret: []byte(receiptSecret)}
}

func (h *Handler) RequireReceiptSecret(c *gin.Context) {
	if len(h.receiptSecret) == 0 {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable,
			gin.H{"error": "receipt signing is not configured", "code": "RECEIPT_DISABLED"})
		return
	}
	c.Next()
}

func (h *Handler) ByCitizen(c *gin.Context) {
	if !authorizeCitizenScope(c, c.Param("citizenId")) {
		return
	}
	entries, err := h.repo.ByCitizen(c.Request.Context(), c.Param("citizenId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit trail", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

func (h *Handler) ByInstitution(c *gin.Context) {
	entries, err := h.repo.ByInstitution(c.Request.Context(), c.Param("institutionId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit trail", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

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
	if !authorizeEntry(c, entry) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entry})
}

func (h *Handler) Receipt(c *gin.Context) {
	entry, err := h.repo.ByID(c.Request.Context(), c.Param("eventId"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "audit entry not found", "code": "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build receipt", "code": "INTERNAL"})
		return
	}
	if !authorizeEntry(c, entry) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": BuildReceipt(entry, h.receiptSecret)})
}

type ReceiptVerifyPayload struct {
	ReceiptID     string      `json:"receiptId" binding:"required"`
	Version       string      `json:"version"`
	SequenceNo    int64       `json:"sequenceNo"`
	EventType     string      `json:"eventType"`
	CitizenID     *string     `json:"citizenId"`
	InstitutionID *string     `json:"institutionId"`
	RequestID     *string     `json:"requestId"`
	Metadata      interface{} `json:"metadata"`
	RecordedAt    string      `json:"recordedAt"`
	PrevHash      string      `json:"prevHash"`
	EntryHash     string      `json:"entryHash"`
	Signature     string      `json:"signature" binding:"required"`
}

func (h *Handler) VerifyReceipt(c *gin.Context) {
	var payload ReceiptVerifyPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "receipt id and signature are required", "code": "INVALID_INPUT"})
		return
	}
	stored, err := h.repo.ByID(c.Request.Context(), payload.ReceiptID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusOK, gin.H{"data": gin.H{"valid": false, "reason": "NOT_ISSUED"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify receipt", "code": "INTERNAL"})
		return
	}
	if !authorizeEntry(c, stored) {
		return
	}
	presented := BuildReceipt(stored, h.receiptSecret)
	if !constantEqual(presented.Signature, payload.Signature) {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"valid": false, "reason": "SIGNATURE_MISMATCH"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"valid":         true,
		"receipt":       presented,
		"chainAnchored": stored.EntryHash == presented.EntryHash,
	}})
}

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

func authorizeCitizenScope(c *gin.Context, citizenID string) bool {
	role := middleware.Role(c.Request.Context())
	if role == string(models.RoleAdmin) {
		return true
	}
	if middleware.SubjectID(c.Request.Context()) != citizenID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "you can only read your own ledger", "code": "FORBIDDEN"})
		return false
	}
	return true
}

func authorizeEntry(c *gin.Context, entry models.AuditEntry) bool {
	role := middleware.Role(c.Request.Context())
	if role == string(models.RoleAdmin) {
		return true
	}
	if role != string(models.RoleCitizen) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "only a citizen can read ledger entries", "code": "FORBIDDEN"})
		return false
	}
	if entry.CitizenID == nil || *entry.CitizenID != middleware.SubjectID(c.Request.Context()) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "this entry belongs to another citizen", "code": "FORBIDDEN"})
		return false
	}
	return true
}

func constantEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

var _ = models.AuditEntry{}
