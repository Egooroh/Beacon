package generic

import (
	"encoding/json"
	"fmt"

	"github.com/Egooroh/beacon/internal/domain"
)

// payload mirrors the JSON structure of the generic Beacon ingest format (§9.1).
type payload struct {
	Level       string            `json:"level"`
	Message     string            `json:"message"`
	Environment string            `json:"environment"`
	Release     string            `json:"release"`
	Exception   *exceptionPayload `json:"exception"`
	Tags        map[string]string `json:"tags"`
}

type exceptionPayload struct {
	Type   string         `json:"type"`
	Value  string         `json:"value"`
	Frames []framePayload `json:"frames"`
}

type framePayload struct {
	Function string `json:"function"`
	Module   string `json:"module"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	InApp    bool   `json:"in_app"`
}

// Parser converts the generic Beacon JSON payload into a domain Event.
type Parser struct{}

// New creates a Parser.
func New() *Parser { return &Parser{} }

// Parse converts raw JSON into a domain Event.
// Returns an error if the JSON is malformed or required fields are absent.
func (p *Parser) Parse(raw []byte) (*domain.Event, error) {
	var pl payload
	if err := json.Unmarshal(raw, &pl); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if pl.Message == "" {
		return nil, fmt.Errorf("message is required")
	}
	if pl.Level == "" {
		pl.Level = string(domain.LevelError)
	}

	ev := &domain.Event{
		Level:       domain.Level(pl.Level),
		Message:     pl.Message,
		Environment: pl.Environment,
		Release:     pl.Release,
		Tags:        pl.Tags,
	}
	if pl.Exception != nil {
		ex := &domain.Exception{
			Type:  pl.Exception.Type,
			Value: pl.Exception.Value,
		}
		for _, f := range pl.Exception.Frames {
			ex.Frames = append(ex.Frames, domain.StackFrame{
				Function: f.Function,
				Module:   f.Module,
				File:     f.File,
				Line:     f.Line,
				InApp:    f.InApp,
			})
		}
		ev.Exception = ex
	}
	return ev, nil
}
