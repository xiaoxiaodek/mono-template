package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestNewWithWriterUsesJSONOutsideDevelopment(t *testing.T) {
	var output bytes.Buffer
	log := NewWithWriter("prod", &output)

	log.Info().Str("request_id", "req_1").Msg("handled")

	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("production log is not JSON: %v", err)
	}
	if event["request_id"] != "req_1" || event["message"] != "handled" {
		t.Fatalf("unexpected structured event: %#v", event)
	}
}

func TestNewWithWriterUsesConsoleOnlyInDevelopment(t *testing.T) {
	var output bytes.Buffer
	log := NewWithWriter("dev", &output)

	log.Debug().Msg("visible")

	if !strings.Contains(output.String(), "visible") {
		t.Fatalf("development console output = %q", output.String())
	}
	if json.Valid(output.Bytes()) {
		t.Fatalf("development output unexpectedly used JSON: %q", output.String())
	}
}

func TestNewWithWriterCanConstructAndTimestampConcurrently(t *testing.T) {
	const loggerCount = 32
	var workers sync.WaitGroup
	workers.Add(loggerCount)
	errors := make(chan error, loggerCount)

	for range loggerCount {
		go func() {
			defer workers.Done()
			var output bytes.Buffer
			log := NewWithWriter("prod", &output)
			log.Info().Msg("concurrent")

			var event map[string]any
			if err := json.Unmarshal(output.Bytes(), &event); err != nil {
				errors <- err
				return
			}
			if event["time"] == nil {
				errors <- errMissingTimestamp{}
			}
		}()
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Errorf("concurrent logger: %v", err)
	}
}

type errMissingTimestamp struct{}

func (errMissingTimestamp) Error() string { return "missing timestamp" }
