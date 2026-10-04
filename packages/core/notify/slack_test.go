package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlack(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	require.NoError(t, Slack(context.Background(), srv.URL, "約定 \"USD_JPY\"\n2行目"))
	assert.Equal(t, "約定 \"USD_JPY\"\n2行目", got["text"])
}

func TestSlack_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("no_service"))
	}))
	defer srv.Close()

	err := Slack(context.Background(), srv.URL, "x")
	assert.ErrorContains(t, err, "404")
	assert.ErrorContains(t, err, "no_service")
}
