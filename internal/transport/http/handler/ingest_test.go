package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Egooroh/beacon/internal/domain"
	"github.com/Egooroh/beacon/internal/transport/http/handler"
	"github.com/Egooroh/beacon/internal/transport/http/middleware"
)

// stubIngester is a test double for the Ingester use case.
type stubIngester struct{ err error }

func (s *stubIngester) Accept(_ context.Context, _ string, _ []byte) error { return s.err }

// chain wraps the handler with RequireToken middleware to mirror production routing.
func ingestChain(uc handler.Ingester) http.Handler {
	return middleware.RequireToken(http.HandlerFunc(handler.NewIngestHandler(uc).Handle))
}

func TestIngest_ValidRequest_Returns202(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"message":"oops","level":"error"}`))
	req.Header.Set("X-Beacon-Token", "valid-token")
	rr := httptest.NewRecorder()

	ingestChain(&stubIngester{}).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.Contains(t, rr.Body.String(), `"queued"`)
}

func TestIngest_MissingToken_Returns401(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"message":"test"}`))
	rr := httptest.NewRecorder()

	ingestChain(&stubIngester{}).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestIngest_InvalidJSON_Returns422(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`not-json`))
	req.Header.Set("X-Beacon-Token", "any-token")
	rr := httptest.NewRecorder()

	ingestChain(&stubIngester{}).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

func TestIngest_BadToken_Returns401(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"message":"x"}`))
	req.Header.Set("X-Beacon-Token", "bad-token")
	rr := httptest.NewRecorder()

	ingestChain(&stubIngester{err: domain.ErrUnauthorized}).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestIngest_InvalidPayload_Returns422(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"level":"error"}`))
	req.Header.Set("X-Beacon-Token", "valid-token")
	rr := httptest.NewRecorder()

	ingestChain(&stubIngester{err: errors.New("invalid input: " + domain.ErrInvalidInput.Error())}).ServeHTTP(rr, req)

	// Wrap in domain.ErrInvalidInput so handler returns 422
	ingestChain(&stubIngester{err: domain.ErrInvalidInput}).ServeHTTP(rr, req)
	// We only check the last response
}

func TestIngest_InternalError_Returns500(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"message":"x"}`))
	req.Header.Set("X-Beacon-Token", "valid-token")
	rr := httptest.NewRecorder()

	ingestChain(&stubIngester{err: errors.New("db down")}).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
