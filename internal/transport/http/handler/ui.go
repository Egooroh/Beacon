package handler

import (
	"net/http"

	"github.com/Egooroh/beacon/internal/transport/http/ui"
)

// NewUIHandler creates an http.Handler that serves the embedded web dashboard.
func NewUIHandler() http.Handler {
	return http.FileServer(http.FS(ui.Files))
}
