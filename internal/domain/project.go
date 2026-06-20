package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Project is a source of errors — one application or service — with its own ingest token.
type Project struct {
	ID        string
	Name      string
	TokenHash string // SHA-256 of the raw ingest token; the raw value is never persisted
	CreatedAt time.Time
}

// HashToken returns the hex-encoded SHA-256 digest of a raw ingest token.
// Always use this before storing or comparing tokens.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
