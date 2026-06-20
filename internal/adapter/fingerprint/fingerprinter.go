package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/Egooroh/beacon/internal/domain"
)

// Fingerprinter computes a deterministic SHA-256 fingerprint per §8.1.
type Fingerprinter struct{}

// New creates a Fingerprinter.
func New() *Fingerprinter { return &Fingerprinter{} }

// Compute returns the fingerprint for ev. The same logical error must always
// produce the same fingerprint regardless of runtime noise (line numbers, IDs).
func (f *Fingerprinter) Compute(e *domain.Event) domain.Fingerprint {
	sig := buildSignature(e)
	h := sha256.Sum256([]byte(sig))
	return domain.Fingerprint(hex.EncodeToString(h[:]))
}

// buildSignature constructs the normalised string that is hashed.
func buildSignature(e *domain.Event) string {
	if e.Exception != nil && len(e.Exception.Frames) > 0 {
		return exceptionSignature(e.Exception)
	}
	return fmt.Sprintf("%s|%s", e.Level, normalizeMessage(e.Message))
}

// exceptionSignature uses InApp frames (falling back to all) and caps at 5.
func exceptionSignature(ex *domain.Exception) string {
	frames := inAppFrames(ex.Frames)
	if len(frames) == 0 {
		frames = ex.Frames
	}
	if len(frames) > 5 {
		frames = frames[:5]
	}
	parts := make([]string, 0, len(frames)+1)
	parts = append(parts, ex.Type)
	for _, f := range frames {
		parts = append(parts, f.Module+":"+f.Function)
	}
	return strings.Join(parts, "|")
}

func inAppFrames(frames []domain.StackFrame) []domain.StackFrame {
	out := make([]domain.StackFrame, 0, len(frames))
	for _, f := range frames {
		if f.InApp {
			out = append(out, f)
		}
	}
	return out
}
