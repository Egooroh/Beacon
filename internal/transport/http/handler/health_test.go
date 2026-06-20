package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Egooroh/beacon/internal/transport/http/handler"
)

type stubPinger struct{ err error }

func (s *stubPinger) Ping(_ context.Context) error { return s.err }

func TestHealthz_AlwaysOK(t *testing.T) {
	h := handler.NewHealthHandler(&stubPinger{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.Healthz(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"ok"`)
}

func TestReadyz_DatabaseReachable(t *testing.T) {
	h := handler.NewHealthHandler(&stubPinger{})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.Readyz(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestReadyz_DatabaseUnreachable(t *testing.T) {
	h := handler.NewHealthHandler(&stubPinger{err: errors.New("connection refused")})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.Readyz(rr, req)

	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), "unavailable")
	// Must not leak internal error details to callers.
	assert.NotContains(t, rr.Body.String(), "connection refused")
}
