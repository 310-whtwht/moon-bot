package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheck(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a new router
	router := gin.New()
	router.GET("/healthz", HealthCheck)

	// Create a test request
	req, err := http.NewRequest("GET", "/healthz", nil)
	assert.NoError(t, err)

	// Create a response recorder
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the response body
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Check required fields
	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "moomoo-trading-api", response["service"])
	assert.Equal(t, "1.0.0", response["version"])

	// Check that system information is present
	system, exists := response["system"].(map[string]interface{})
	assert.True(t, exists)
	assert.Contains(t, system, "goroutines")
	assert.Contains(t, system, "memory")

	// Check that checks are present
	checks, exists := response["checks"].(map[string]interface{})
	assert.True(t, exists)
	assert.Equal(t, "healthy", checks["database"])
	assert.Equal(t, "healthy", checks["redis"])
	assert.Equal(t, "connected", checks["broker"])

	// Check that metrics are present
	metrics, exists := response["metrics"].(map[string]interface{})
	assert.True(t, exists)
	assert.Contains(t, metrics, "lag")
	assert.Contains(t, metrics, "queue_size")
	assert.Contains(t, metrics, "reconnect_count")
	assert.Contains(t, metrics, "vm_quota_hits")
}