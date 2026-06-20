package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Egooroh/beacon/internal/transport/http/middleware"
)

func TestRequireToken_MissingHeader_Returns401(t *testing.T) {
	handler := middleware.RequireToken(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/ingest", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireToken_WithHeader_CallsNext(t *testing.T) {
	handler := middleware.RequireToken(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/ingest", nil)
	req.Header.Set("X-Beacon-Token", "my-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTokenFromContext_ReturnsTokenSetByMiddleware(t *testing.T) {
	var captured string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = middleware.TokenFromContext(r.Context())
	})
	handler := middleware.RequireToken(next)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Beacon-Token", "secret-token")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "secret-token", captured)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
