package domain

import "time"

// Level is the severity of an error event.
type Level string

const (
	LevelDebug   Level = "debug"
	LevelInfo    Level = "info"
	LevelWarning Level = "warning"
	LevelError   Level = "error"
	LevelFatal   Level = "fatal"
)

// Fingerprint is a hex-encoded SHA-256 hash identifying an error class.
type Fingerprint string

// Event is a single occurrence of an error received from an application.
type Event struct {
	ID          string
	ProjectID   string
	Fingerprint Fingerprint
	Level       Level
	Message     string
	Exception   *Exception
	Environment string
	Release     string
	Tags        map[string]string
	RawPayload  []byte
	ReceivedAt  time.Time
}

// Exception holds the error type and its stack trace.
type Exception struct {
	Type   string
	Value  string
	Frames []StackFrame
}

// StackFrame is one frame in a stack trace.
type StackFrame struct {
	Function string
	Module   string
	File     string
	Line     int
	InApp    bool // true when the frame belongs to application code, not vendor/runtime
}
