package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// refreshCookieName is the httpOnly cookie that carries the refresh token.
const refreshCookieName = "yomi_refresh"

// isUniqueViolation reports whether err is a PostgreSQL unique constraint
// violation, which we map to a 409 Conflict.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// bcryptCompare reports whether password matches the stored hash.
func bcryptCompare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// setRefreshCookie writes the refresh token as an httpOnly, same-site cookie.
func setRefreshCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, token, int((7 * 24 * time.Hour).Seconds()), "/", "", true, true)
}

// refreshTokenFromCookie reads the refresh token from the cookie jar.
func refreshTokenFromCookie(c *gin.Context) string {
	token, err := c.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return token
}

// clearRefreshCookie expires the refresh cookie.
func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, "", -1, "/", "", true, true)
}
