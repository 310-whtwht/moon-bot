package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireToken rejects requests that do not carry `Authorization: Bearer <token>`.
//
// The API is only meant to be called by the web app, which adds the token
// server-side after checking the user's session. The token is compared in
// constant time.
func RequireToken(token string) gin.HandlerFunc {
	want := []byte(token)
	return func(c *gin.Context) {
		got, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			c.Header("WWW-Authenticate", `Bearer realm="api"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	}
}
