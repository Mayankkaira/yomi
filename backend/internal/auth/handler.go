package auth

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/yomi/yomi/internal/models"
)

// Handler exposes the Auth HTTP API.
type Handler struct {
	repo  *Repository
	token *TokenService
}

// NewHandler wires the repository and token service into an HTTP handler.
func NewHandler(repo *Repository, token *TokenService) *Handler {
	return &Handler{repo: repo, token: token}
}

// RegisterRequest is the payload for POST /auth/register.
type RegisterRequest struct {
	FullName  string `json:"fullName" binding:"required"`
	AadhaarID string `json:"aadhaarId" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Phone     string `json:"phone"`
	Password  string `json:"password" binding:"required,min=8"`
}

var aadhaarRe = regexp.MustCompile(`^[0-9]{12}$`)

// Register creates a citizen account.
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid registration payload", "code": "INVALID_INPUT"})
		return
	}
	if !aadhaarRe.MatchString(req.AadhaarID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Aadhaar ID must be exactly 12 digits", "code": "INVALID_AADHAAR"})
		return
	}
	citizen, err := h.repo.CreateCitizen(c.Request.Context(), req.FullName, req.AadhaarID, req.Email, req.Phone, req.Password)
	if err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "Aadhaar ID or email already registered", "code": "CONFLICT"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account", "code": "INTERNAL"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": citizen})
}

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	AadhaarID string `json:"aadhaarId" binding:"required"`
	Password  string `json:"password" binding:"required"`
}

// Login verifies credentials and issues access + refresh tokens.
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid login payload", "code": "INVALID_INPUT"})
		return
	}
	citizen, err := h.repo.CitizenByAadhaar(c.Request.Context(), req.AadhaarID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Don't reveal whether the account exists.
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials", "code": "INVALID_CREDENTIALS"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed", "code": "INTERNAL"})
		return
	}
	if err := bcryptCompare(citizen.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials", "code": "INVALID_CREDENTIALS"})
		return
	}

	access, err := h.token.IssueAccessToken(citizen.ID, models.RoleCitizen)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issuance failed", "code": "INTERNAL"})
		return
	}
	refresh, err := h.token.IssueRefreshToken(c.Request.Context(), citizen.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issuance failed", "code": "INTERNAL"})
		return
	}
	setRefreshCookie(c, refresh)
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"accessToken": access,
			"citizen":     citizen,
		},
	})
}

// RefreshRequest is the payload for POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// Refresh rotates a valid refresh token and returns a fresh access token. The
// token may arrive in the JSON body or in the httpOnly cookie; an empty body
// is accepted when the cookie is present.
func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	bindErr := c.ShouldBindJSON(&req)
	raw := req.RefreshToken
	if raw == "" {
		raw = refreshTokenFromCookie(c)
	}
	if raw == "" {
		// No token anywhere: distinguish a malformed body from a missing token.
		if bindErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid refresh payload", "code": "INVALID_INPUT"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token", "code": "UNAUTHORIZED"})
		return
	}
	subject, err := h.token.ValidateRefreshToken(c.Request.Context(), raw)
	if err != nil {
		if errors.Is(err, ErrInvalidRefresh) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token invalid or expired", "code": "UNAUTHORIZED"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refresh failed", "code": "INTERNAL"})
		return
	}
	citizen, err := h.repo.CitizenByID(c.Request.Context(), subject)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "account not found", "code": "UNAUTHORIZED"})
		return
	}
	access, err := h.token.IssueAccessToken(subject, models.RoleCitizen)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issuance failed", "code": "INTERNAL"})
		return
	}
	newRefresh, err := h.token.IssueRefreshToken(c.Request.Context(), subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issuance failed", "code": "INTERNAL"})
		return
	}
	setRefreshCookie(c, newRefresh)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"accessToken": access, "citizen": citizen}})
}

// LogoutRequest is the payload for POST /auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// Logout invalidates the presented refresh token.
func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest
	_ = c.ShouldBindJSON(&req)
	raw := req.RefreshToken
	if raw == "" {
		raw = refreshTokenFromCookie(c)
	}
	if raw != "" {
		if err := h.token.RevokeRefreshToken(c.Request.Context(), raw); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed", "code": "INTERNAL"})
			return
		}
	}
	clearRefreshCookie(c)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "logged_out"}})
}
