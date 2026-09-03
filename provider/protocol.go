package provider

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	FrameRequest = "request"
	FrameEvent   = "event"
	FrameResult  = "result"
)

// Request is the single JSONL frame written to provider standard input.
type Request struct {
	Version    string          `json:"version"`
	Kind       string          `json:"kind"`
	RequestID  string          `json:"requestId"`
	Capability string          `json:"capability"`
	Operation  string          `json:"operation,omitempty"`
	Context    map[string]any  `json:"context,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
}

// Event is a streaming JSONL frame emitted before the final result.
type Event struct {
	Version   string          `json:"version"`
	Kind      string          `json:"kind"`
	RequestID string          `json:"requestId"`
	Event     string          `json:"event"`
	Message   string          `json:"message,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// ResultStatus is the semantic outcome of a provider action.
type ResultStatus string

const (
	ResultOK       ResultStatus = "ok"
	ResultDeclined ResultStatus = "declined"
	ResultError    ResultStatus = "error"
)

// Result is the final JSONL frame emitted by a provider.
type Result struct {
	Version   string            `json:"version"`
	Kind      string            `json:"kind"`
	RequestID string            `json:"requestId"`
	Status    ResultStatus      `json:"status"`
	Output    json.RawMessage   `json:"output,omitempty"`
	Message   string            `json:"message,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func prepareRequest(request Request) (Request, error) {
	if request.Version == "" {
		request.Version = Version
	}
	if request.Kind == "" {
		request.Kind = FrameRequest
	}
	if request.Version != Version {
		return request, fmt.Errorf("request version must be %q", Version)
	}
	if request.Kind != FrameRequest {
		return request, fmt.Errorf("request kind must be %q", FrameRequest)
	}
	if request.RequestID == "" {
		return request, errors.New("request id is required")
	}
	if request.Capability == "" {
		return request, errors.New("request capability is required")
	}
	if len(request.Input) > 0 && !json.Valid(request.Input) {
		return request, errors.New("request input must be valid JSON")
	}
	return request, nil
}

func validateEvent(event Event, requestID string) error {
	if event.Version != Version || event.Kind != FrameEvent {
		return errors.New("invalid event frame version or kind")
	}
	if event.RequestID != requestID {
		return fmt.Errorf("event request id %q does not match %q", event.RequestID, requestID)
	}
	if event.Event == "" {
		return errors.New("event name is required")
	}
	if len(event.Data) > 0 && !json.Valid(event.Data) {
		return errors.New("event data must be valid JSON")
	}
	return nil
}

func validateResult(result Result, requestID string) error {
	if result.Version != Version || result.Kind != FrameResult {
		return errors.New("invalid result frame version or kind")
	}
	if result.RequestID != requestID {
		return fmt.Errorf("result request id %q does not match %q", result.RequestID, requestID)
	}
	switch result.Status {
	case ResultOK, ResultDeclined, ResultError:
	default:
		return fmt.Errorf("invalid result status %q", result.Status)
	}
	if len(result.Output) > 0 && !json.Valid(result.Output) {
		return errors.New("result output must be valid JSON")
	}
	return nil
}
