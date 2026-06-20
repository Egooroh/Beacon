package middleware

import (
	"context"
	"net/http"
)

type tokenCtxKey struct{}

// RequireToken rejects requests that are missing the X-Beacon-Token header.
// Actual token validation (DB lookup) is deferred to the use case (FR-4).
func RequireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Beacon-Token")
		if token == "" {
			http.Error(w, "X-Beacon-Token header is required", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), tokenCtxKey{}, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TokenFromContext retrieves the ingest token previously set by RequireToken.
// Returns an empty string when called outside of the RequireToken middleware.
func TokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(tokenCtxKey{}).(string)
	return token
}
