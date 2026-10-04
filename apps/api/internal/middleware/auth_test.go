package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequireToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	api := r.Group("/api/v1", RequireToken("s3cret-token"))
	api.PUT("/kill-switch/:scope", func(c *gin.Context) { c.String(http.StatusOK, "changed") })

	call := func(method, path, authorization string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	assert.Equal(t, http.StatusOK, call("GET", "/healthz", "").Code, "health check stays open")

	for name, header := range map[string]string{
		"no header":        "",
		"wrong token":      "Bearer nope",
		"prefix of token":  "Bearer s3cret",
		"token with extra": "Bearer s3cret-token-and-more",
		"wrong scheme":     "Basic s3cret-token",
		"bare token":       "s3cret-token",
		"lowercase scheme": "bearer s3cret-token",
	} {
		t.Run(name, func(t *testing.T) {
			w := call("PUT", "/api/v1/kill-switch/global", header)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.NotContains(t, w.Body.String(), "changed", "the handler must not run")
			assert.NotEmpty(t, w.Header().Get("WWW-Authenticate"))
		})
	}

	w := call("PUT", "/api/v1/kill-switch/global", "Bearer s3cret-token")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "changed", w.Body.String())
}
