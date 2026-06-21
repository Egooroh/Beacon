package sentry

import (
	"encoding/json"
	"fmt"

	"github.com/Egooroh/beacon/internal/domain"
)

// payload is the top-level Sentry error webhook structure.
type payload struct {
	Data struct {
		Event event `json:"event"`
	} `json:"data"`
}

type event struct {
	Level       string     `json:"level"`
	Message     string     `json:"message"`
	LogEntry    *logEntry  `json:"logentry"`
	Exception   *exception `json:"exception"`
	Tags        [][]string `json:"tags"`
	Environment string     `json:"environment"`
	Release     string     `json:"release"`
}

type logEntry struct {
	Message string `json:"message"`
}

type exception struct {
	Values []excValue `json:"values"`
}

type excValue struct {
	Type       string      `json:"type"`
	Value      string      `json:"value"`
	Stacktrace *stacktrace `json:"stacktrace"`
}

type stacktrace struct {
	Frames []frame `json:"frames"`
}

type frame struct {
	Filename string `json:"filename"`
	Function string `json:"function"`
	Module   string `json:"module"`
	Lineno   int    `json:"lineno"`
	InApp    bool   `json:"in_app"`
}

// Parser converts Sentry webhook payloads into domain Events.
type Parser struct{}

// New creates a Sentry Parser.
func New() *Parser { return &Parser{} }

// Parse maps a Sentry webhook JSON body to a domain.Event.
// The outermost exception value is used when present.
func (p *Parser) Parse(raw []byte) (*domain.Event, error) {
	var pl payload
	if err := json.Unmarshal(raw, &pl); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	ev := pl.Data.Event

	msg := ev.Message
	if msg == "" && ev.LogEntry != nil {
		msg = ev.LogEntry.Message
	}
	if msg == "" && ev.Exception != nil && len(ev.Exception.Values) > 0 {
		msg = ev.Exception.Values[len(ev.Exception.Values)-1].Value
	}
	if msg == "" {
		return nil, fmt.Errorf("message is required")
	}

	level := ev.Level
	if level == "" {
		level = string(domain.LevelError)
	}

	domainEv := &domain.Event{
		Level:       domain.Level(level),
		Message:     msg,
		Environment: ev.Environment,
		Release:     ev.Release,
		Tags:        tagsToMap(ev.Tags),
		RawPayload:  raw,
	}

	if ev.Exception != nil && len(ev.Exception.Values) > 0 {
		exc := ev.Exception.Values[len(ev.Exception.Values)-1]
		de := &domain.Exception{
			Type:  exc.Type,
			Value: exc.Value,
		}
		if exc.Stacktrace != nil {
			for _, f := range exc.Stacktrace.Frames {
				de.Frames = append(de.Frames, domain.StackFrame{
					Function: f.Function,
					Module:   f.Module,
					File:     f.Filename,
					Line:     f.Lineno,
					InApp:    f.InApp,
				})
			}
		}
		domainEv.Exception = de
	}

	return domainEv, nil
}

func tagsToMap(tags [][]string) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	m := make(map[string]string, len(tags))
	for _, pair := range tags {
		if len(pair) == 2 {
			m[pair[0]] = pair[1]
		}
	}
	return m
}
