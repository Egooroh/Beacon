package sentry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Egooroh/beacon/internal/adapter/ingest/sentry"
	"github.com/Egooroh/beacon/internal/domain"
)

const exceptionPayload = `{
  "data": {
    "event": {
      "level": "error",
      "message": "card declined",
      "exception": {
        "values": [{
          "type": "stripe.CardError",
          "value": "card declined",
          "stacktrace": {
            "frames": [
              {"function":"Do","module":"net/http","filename":"client.go","lineno":601,"in_app":false},
              {"function":"ChargeCard","module":"billing","filename":"charge.go","lineno":88,"in_app":true}
            ]
          }
        }]
      },
      "tags": [["env","prod"],["user","42"]],
      "environment": "production",
      "release": "api@1.4.2"
    }
  }
}`

func TestParse_ExceptionEvent(t *testing.T) {
	p := sentry.New()
	ev, err := p.Parse([]byte(exceptionPayload))

	require.NoError(t, err)
	assert.Equal(t, domain.LevelError, ev.Level)
	assert.Equal(t, "card declined", ev.Message)
	assert.Equal(t, "production", ev.Environment)
	assert.Equal(t, "api@1.4.2", ev.Release)
	assert.Equal(t, map[string]string{"env": "prod", "user": "42"}, ev.Tags)

	require.NotNil(t, ev.Exception)
	assert.Equal(t, "stripe.CardError", ev.Exception.Type)
	require.Len(t, ev.Exception.Frames, 2)
	assert.Equal(t, "ChargeCard", ev.Exception.Frames[1].Function)
	assert.True(t, ev.Exception.Frames[1].InApp)
}

func TestParse_LogEntryFallback(t *testing.T) {
	raw := `{"data":{"event":{"level":"warning","logentry":{"message":"disk almost full"}}}}`
	ev, err := sentry.New().Parse([]byte(raw))

	require.NoError(t, err)
	assert.Equal(t, domain.LevelWarning, ev.Level)
	assert.Equal(t, "disk almost full", ev.Message)
	assert.Nil(t, ev.Exception)
}

func TestParse_DefaultLevel(t *testing.T) {
	raw := `{"data":{"event":{"message":"hello"}}}`
	ev, err := sentry.New().Parse([]byte(raw))

	require.NoError(t, err)
	assert.Equal(t, domain.LevelError, ev.Level)
}

func TestParse_MissingMessage_ReturnsError(t *testing.T) {
	raw := `{"data":{"event":{"level":"error"}}}`
	_, err := sentry.New().Parse([]byte(raw))
	assert.Error(t, err)
}

func TestParse_InvalidJSON_ReturnsError(t *testing.T) {
	_, err := sentry.New().Parse([]byte(`not json`))
	assert.Error(t, err)
}
