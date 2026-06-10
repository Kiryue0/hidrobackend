package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kiryue0/hidrobackend/internal/app/ports"
)

const ctxUserIDKey = "userID"

// AuthMiddleware Authorization: Bearer <token> başlığını doğrular ve
// userID'yi gin context'e koyar.
func AuthMiddleware(tokens ports.TokenIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token gerekli"})
			return
		}
		token := strings.TrimSpace(h[len(prefix):])
		userID, err := tokens.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "geçersiz token"})
			return
		}
		c.Set(ctxUserIDKey, userID)
		c.Next()
	}
}

// userIDFrom context'e konmuş userID'yi döner.
func userIDFrom(c *gin.Context) int64 {
	v, ok := c.Get(ctxUserIDKey)
	if !ok {
		return 0
	}
	id, _ := v.(int64)
	return id
}
