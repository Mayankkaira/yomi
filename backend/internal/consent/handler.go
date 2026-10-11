package consent

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yomi/yomi/internal/instclient"
	"github.com/yomi/yomi/internal/middleware"
	"github.com/yomi/yomi/internal/models"
)

type Handler struct {
	svc  *Service
	inst *instclient.Client
}

func NewHandler(svc *Service, inst *instclient.Client) *Handler {
	return &Handler{svc: svc, inst: inst}
}

type RequestPayload struct {
	InstitutionID string    `json:"institutionId"`
	CitizenID     string    `json:"citizenId" binding:"required"`
	DataScope     []string  `json:"dataScope" binding:"required,min=1"`
	Purpose       string    `json:"purpose" binding:"required"`
	ExpiresAt     time.Time `json:"expiresAt" binding:"required"`
}

func (h *Handler) Request(c *gin.Context) {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing institution api key", "code": "UNAUTHORIZED"})
		return
	}

	inst, err := h.inst.Authenticate(c.Request.Context(), apiKey)
	if err != nil {
		if errors.Is(err, instclient.ErrInvalidKey) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid institution api key", "code": "INVALID_API_KEY"})
			return
		}
		slog.Error("institution authentication failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "institution verification unavailable", "code": "SERVICE_UNAVAILABLE"})
		return
	}

	var payload RequestPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid consent request payload", "code": "INVALID_INPUT"})
		return
	}
	if payload.InstitutionID != "" && payload.InstitutionID != inst.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "institution id does not match api key", "code": "FORBIDDEN"})
		return
	}
	if time.Now().After(payload.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expiry must be in the future", "code": "INVALID_EXPIRY"})
		return
	}
	req, err := h.svc.Request(c.Request.Context(), RequestInput{
		CitizenID:     payload.CitizenID,
		InstitutionID: inst.ID,
		Role:          inst.Role,
		Scope:         payload.DataScope,
		Purpose:       payload.Purpose,
		ExpiresAt:     payload.ExpiresAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrUnknownScope):
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown data scope requested", "code": "UNKNOWN_SCOPE"})
		case errors.Is(err, ErrScopeNotAllowed):
			c.JSON(http.StatusForbidden, gin.H{"error": "institution role cannot request this data scope", "code": "SCOPE_NOT_ALLOWED"})
		case errors.Is(err, ErrUnknownRole):
			c.JSON(http.StatusForbidden, gin.H{"error": "institution has no valid role", "code": "UNKNOWN_ROLE"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create consent request", "code": "INTERNAL"})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": req})
}

func (h *Handler) Pending(c *gin.Context) {
	citizenID := middleware.SubjectID(c.Request.Context())
	if citizenID == "" {
		citizenID = c.Param("citizenId")
	}
	reqs, err := h.svc.repo.PendingByCitizen(c.Request.Context(), citizenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list pending requests", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": reqs})
}

func (h *Handler) Approve(c *gin.Context) {
	requestID := c.Param("requestId")
	citizenID := middleware.SubjectID(c.Request.Context())
	req, token, err := h.svc.Approve(c.Request.Context(), requestID, citizenID)
	if err != nil {
		writeConsentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"request": req, "accessToken": token}})
}

func (h *Handler) Reject(c *gin.Context) {
	requestID := c.Param("requestId")
	citizenID := middleware.SubjectID(c.Request.Context())
	req, err := h.svc.Reject(c.Request.Context(), requestID, citizenID)
	if err != nil {
		writeConsentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": req})
}

func (h *Handler) Revoke(c *gin.Context) {
	requestID := c.Param("requestId")
	citizenID := middleware.SubjectID(c.Request.Context())
	req, err := h.svc.Revoke(c.Request.Context(), requestID, citizenID)
	if err != nil {
		writeConsentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": req})
}

func (h *Handler) History(c *gin.Context) {
	citizenID := middleware.SubjectID(c.Request.Context())
	reqs, err := h.svc.repo.HistoryByCitizen(c.Request.Context(), citizenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load consent history", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": reqs})
}

func (h *Handler) ScopeCatalog(c *gin.Context) {
	role := models.InstitutionRole(c.Query("role"))
	if role == "" {
		role = models.InstitutionRoleViewer
	}
	if !ValidRole(role) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown institution role", "code": "UNKNOWN_ROLE"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"role": role, "scopes": CatalogForRole(role)}})
}

type VerifyTokenPayload struct {
	AccessToken string `json:"accessToken" binding:"required"`
}

func (h *Handler) VerifyToken(c *gin.Context) {
	var payload VerifyTokenPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "accessToken is required", "code": "INVALID_INPUT"})
		return
	}
	auth, err := h.svc.VerifyAccessToken(c.Request.Context(), payload.AccessToken)
	if err != nil {
		reason := "INVALID"
		switch {
		case errors.Is(err, ErrTokenExpired):
			reason = "EXPIRED"
		case errors.Is(err, ErrTokenRevoked):
			reason = "REVOKED"
		}
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"active": false, "reason": reason}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"active":        true,
		"requestId":     auth.RequestID,
		"institutionId": auth.InstitutionID,
		"scope":         auth.Scope,
		"expiresAt":     auth.ExpiresAt,
	}})
}

func writeConsentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "consent request not found", "code": "NOT_FOUND"})
	case errors.Is(err, ErrNotOwned):
		c.JSON(http.StatusForbidden, gin.H{"error": "not your consent request", "code": "FORBIDDEN"})
	case errors.Is(err, ErrInvalidState):
		c.JSON(http.StatusConflict, gin.H{"error": "consent request is not in a valid state", "code": "INVALID_STATE"})
	case errors.Is(err, ErrExpired):
		c.JSON(http.StatusConflict, gin.H{"error": "consent request has expired", "code": "EXPIRED"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": "INTERNAL"})
	}
}
