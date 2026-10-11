package institution

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yomi/yomi/internal/models"
)

// Handler exposes the Institution HTTP API.
type Handler struct {
	repo *Repository
}

// NewHandler wires the repository into an HTTP handler.
func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// AuthenticatePayload is the body for POST /institution/authenticate.
type AuthenticatePayload struct {
	APIKey string `json:"apiKey" binding:"required"`
}

// Authenticate validates an institution API key. This endpoint is restricted
// to internal service-to-service calls via the shared internal secret header.
func (h *Handler) Authenticate(c *gin.Context) {
	var payload AuthenticatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid authenticate payload", "code": "INVALID_INPUT"})
		return
	}
	inst, err := h.repo.Authenticate(c.Request.Context(), payload.APIKey)
	if err != nil {
		if errors.Is(err, ErrInvalidKey) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid institution api key", "code": "INVALID_API_KEY"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication failed", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": inst})
}

// RequireInternalSecret guards internal-only endpoints using a shared secret
// carried in the X-Yomi-Internal header.
func RequireInternalSecret(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" || c.GetHeader("X-Yomi-Internal") != secret {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "code": "FORBIDDEN"})
			return
		}
		c.Next()
	}
}

// RegisterPayload is the body for POST /institution/register.
type RegisterPayload struct {
	Name   string `json:"name" binding:"required"`
	Sector string `json:"sector" binding:"required"`
	Role   string `json:"role"`
}

// Register creates an institution and returns the one-time plaintext API key.
// The role defaults to viewer, the lowest trust tier, so a caller must ask
// explicitly for a wider data ceiling.
func (h *Handler) Register(c *gin.Context) {
	var payload RegisterPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid institution payload", "code": "INVALID_INPUT"})
		return
	}
	role := models.InstitutionRole(payload.Role)
	if role == "" {
		role = models.InstitutionRoleViewer
	}
	inst, apiKey, err := h.repo.Create(c.Request.Context(), payload.Name, payload.Sector, role)
	if err != nil {
		if errors.Is(err, ErrInvalidRole) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown institution role", "code": "INVALID_ROLE"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register institution", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": inst, "apiKey": apiKey})
}

// SetRole changes an institution's role, which caps the data scopes it may
// request. Existing approvals are not widened: the ceiling applies to new
// requests only.
func (h *Handler) SetRole(c *gin.Context) {
	var payload struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role is required", "code": "INVALID_INPUT"})
		return
	}
	inst, err := h.repo.SetRole(c.Request.Context(), c.Param("id"), models.InstitutionRole(payload.Role))
	if err != nil {
		if errors.Is(err, ErrInvalidRole) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown institution role", "code": "INVALID_ROLE"})
			return
		}
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "institution not found", "code": "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": inst})
}

// Get returns an institution by ID.
func (h *Handler) Get(c *gin.Context) {
	inst, err := h.repo.ByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "institution not found", "code": "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load institution", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": inst})
}

// RotateKey rotates an institution's API key and returns the new plaintext.
func (h *Handler) RotateKey(c *gin.Context) {
	inst, apiKey, err := h.repo.RotateKey(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "institution not found", "code": "NOT_FOUND"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate API key", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": inst, "apiKey": apiKey})
}
