package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Egooroh/beacon/internal/adapter/ingest/generic"
	"github.com/Egooroh/beacon/internal/domain"
)

var parser = generic.New()

func TestParse_FullPayload(t *testing.T) {
	raw := []byte(`{
		"level": "error",
		"message": "failed to charge card",
		"environment": "production",
		"release": "api@1.4.2",
		"exception": {
			"type": "*stripe.CardError",
			"value": "card declined",
			"frames": [
				{"function":"ChargeCard","module":"billing","file":"charge.go","line":88,"in_app":true},
				{"function":"Do","module":"net/http","file":"client.go","line":601,"in_app":false}
			]
		},
		"tags": {"user_id":"42"}
	}`)

	ev, err := parser.Parse(raw)
	require.NoError(t, err)

	assert.Equal(t, domain.LevelError, ev.Level)
	assert.Equal(t, "failed to charge card", ev.Message)
	assert.Equal(t, "production", ev.Environment)
	assert.Equal(t, "api@1.4.2", ev.Release)
	assert.Equal(t, map[string]string{"user_id": "42"}, ev.Tags)

	require.NotNil(t, ev.Exception)
	assert.Equal(t, "*stripe.CardError", ev.Exception.Type)
	assert.Equal(t, "card declined", ev.Exception.Value)
	require.Len(t, ev.Exception.Frames, 2)
	assert.True(t, ev.Exception.Frames[0].InApp)
	assert.Equal(t, 88, ev.Exception.Frames[0].Line)
}

func TestParse_DefaultsLevelToError(t *testing.T) {
	ev, err := parser.Parse([]byte(`{"message":"test"}`))
	require.NoError(t, err)
	assert.Equal(t, domain.LevelError, ev.Level)
}

func TestParse_MissingMessage_ReturnsError(t *testing.T) {
	_, err := parser.Parse([]byte(`{"level":"error"}`))
	assert.Error(t, err)
}

func TestParse_InvalidJSON_ReturnsError(t *testing.T) {
	_, err := parser.Parse([]byte(`not json`))
	assert.Error(t, err)
}

func TestParse_NoException_ReturnsNilException(t *testing.T) {
	ev, err := parser.Parse([]byte(`{"message":"hello","level":"info"}`))
	require.NoError(t, err)
	assert.Nil(t, ev.Exception)
}
