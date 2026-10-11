package consent

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RequireInternalSecret(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" || c.GetHeader("X-Yomi-Internal") != secret {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "code": "FORBIDDEN"})
			return
		}
		c.Next()
	}
}
