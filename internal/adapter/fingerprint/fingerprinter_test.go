package fingerprint_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Egooroh/beacon/internal/adapter/fingerprint"
	"github.com/Egooroh/beacon/internal/domain"
)

func frames(inApp bool, pairs ...string) []domain.StackFrame {
	fs := make([]domain.StackFrame, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		fs = append(fs, domain.StackFrame{Module: pairs[i], Function: pairs[i+1], InApp: inApp})
	}
	return fs
}

// ── exception-based fingerprints ─────────────────────────────────────────────

func TestCompute_Exception_Deterministic(t *testing.T) {
	fp := fingerprint.New()
	ev := &domain.Event{
		Level: domain.LevelError,
		Exception: &domain.Exception{
			Type:   "NullPointerException",
			Frames: frames(true, "com.app", "handler", "com.app", "service"),
		},
	}
	a := fp.Compute(ev)
	b := fp.Compute(ev)
	assert.Equal(t, a, b)
	assert.Len(t, string(a), 64) // sha256 hex
}

func TestCompute_Exception_DifferentType_DifferentFingerprint(t *testing.T) {
	fp := fingerprint.New()
	base := []domain.StackFrame{{Module: "app", Function: "Run", InApp: true}}
	ev1 := &domain.Event{Exception: &domain.Exception{Type: "TypeError", Frames: base}}
	ev2 := &domain.Event{Exception: &domain.Exception{Type: "ValueError", Frames: base}}
	assert.NotEqual(t, fp.Compute(ev1), fp.Compute(ev2))
}

func TestCompute_Exception_InAppFramesTakePriority(t *testing.T) {
	fp := fingerprint.New()
	ev1 := &domain.Event{
		Exception: &domain.Exception{
			Type: "Err",
			Frames: []domain.StackFrame{
				{Module: "vendor", Function: "foo", InApp: false},
				{Module: "app", Function: "bar", InApp: true},
			},
		},
	}
	// ev2 has only the inApp frame — fingerprint must match ev1.
	ev2 := &domain.Event{
		Exception: &domain.Exception{
			Type:   "Err",
			Frames: []domain.StackFrame{{Module: "app", Function: "bar", InApp: true}},
		},
	}
	assert.Equal(t, fp.Compute(ev1), fp.Compute(ev2))
}

func TestCompute_Exception_FallbackToAllFrames_WhenNoInApp(t *testing.T) {
	fp := fingerprint.New()
	ev := &domain.Event{
		Exception: &domain.Exception{
			Type:   "Err",
			Frames: frames(false, "vendor", "libFoo", "vendor", "libBar"),
		},
	}
	// Must not panic and must return a valid fingerprint.
	result := fp.Compute(ev)
	require.Len(t, string(result), 64)
}

func TestCompute_Exception_MaxFiveFrames(t *testing.T) {
	fp := fingerprint.New()
	many := make([]domain.StackFrame, 10)
	for i := range many {
		many[i] = domain.StackFrame{Module: "app", Function: "fn", InApp: true}
	}
	five := make([]domain.StackFrame, 5)
	copy(five, many[:5])

	evMany := &domain.Event{Exception: &domain.Exception{Type: "E", Frames: many}}
	evFive := &domain.Event{Exception: &domain.Exception{Type: "E", Frames: five}}
	assert.Equal(t, fp.Compute(evFive), fp.Compute(evMany),
		"top-5 frames must produce the same fingerprint as 10 frames")
}

// ── message-based fingerprints ────────────────────────────────────────────────

func TestCompute_Message_Deterministic(t *testing.T) {
	fp := fingerprint.New()
	ev := &domain.Event{Level: domain.LevelError, Message: "connection refused on port 5432"}
	assert.Equal(t, fp.Compute(ev), fp.Compute(ev))
}

func TestCompute_Message_DifferentNumbers_SameFingerprint(t *testing.T) {
	fp := fingerprint.New()
	ev1 := &domain.Event{Level: domain.LevelError, Message: "user 42 not found"}
	ev2 := &domain.Event{Level: domain.LevelError, Message: "user 9981 not found"}
	assert.Equal(t, fp.Compute(ev1), fp.Compute(ev2))
}

func TestCompute_Message_DifferentUUIDs_SameFingerprint(t *testing.T) {
	fp := fingerprint.New()
	ev1 := &domain.Event{Level: domain.LevelError, Message: "record 550e8400-e29b-41d4-a716-446655440000 missing"}
	ev2 := &domain.Event{Level: domain.LevelError, Message: "record 6ba7b810-9dad-11d1-80b4-00c04fd430c8 missing"}
	assert.Equal(t, fp.Compute(ev1), fp.Compute(ev2))
}

func TestCompute_Message_DifferentLevels_DifferentFingerprint(t *testing.T) {
	fp := fingerprint.New()
	ev1 := &domain.Event{Level: domain.LevelError, Message: "disk full"}
	ev2 := &domain.Event{Level: domain.LevelWarning, Message: "disk full"}
	assert.NotEqual(t, fp.Compute(ev1), fp.Compute(ev2))
}

// ── normalizeMessage ──────────────────────────────────────────────────────────

// normalizeMessage is package-private; test indirectly via Compute.
func TestCompute_NormalizesURL(t *testing.T) {
	fp := fingerprint.New()
	ev1 := &domain.Event{Level: domain.LevelError, Message: "failed to fetch https://api.example.com/v1/users"}
	ev2 := &domain.Event{Level: domain.LevelError, Message: "failed to fetch https://other.host.io/path?q=1"}
	assert.Equal(t, fp.Compute(ev1), fp.Compute(ev2))
}

func TestCompute_NormalizesEmail(t *testing.T) {
	fp := fingerprint.New()
	ev1 := &domain.Event{Level: domain.LevelError, Message: "user alice@example.com not found"}
	ev2 := &domain.Event{Level: domain.LevelError, Message: "user bob@other.org not found"}
	assert.Equal(t, fp.Compute(ev1), fp.Compute(ev2))
}

func TestCompute_NormalizesHexAddress(t *testing.T) {
	fp := fingerprint.New()
	ev1 := &domain.Event{Level: domain.LevelFatal, Message: "SIGSEGV at 0xdeadbeef"}
	ev2 := &domain.Event{Level: domain.LevelFatal, Message: "SIGSEGV at 0xcafebabe"}
	assert.Equal(t, fp.Compute(ev1), fp.Compute(ev2))
}

func TestCompute_NormalizesQuotedStrings(t *testing.T) {
	fp := fingerprint.New()
	ev1 := &domain.Event{Level: domain.LevelError, Message: `column "user_id" does not exist`}
	ev2 := &domain.Event{Level: domain.LevelError, Message: `column "email" does not exist`}
	assert.Equal(t, fp.Compute(ev1), fp.Compute(ev2))
}
