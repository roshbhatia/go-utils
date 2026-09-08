package paths

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/roshbhatia/go-utils/xdg"
)

const (
	StateHomeKey        = "stateHome"
	AgentsKey           = "agents"
	AgentPanesKey       = "agentPanes"
	AgentDiffNotesKey   = "agentDiffNotes"
	AgentEditsKey       = "agentEdits"
	AgentWtrunKey       = "agentWtrun"
	AgentWorkerKey      = "agentWorker"
	AgentTranscriptsKey = "agentTranscripts"
	AgentWorklogKey     = "agentWorklog"
	SeshySessionsKey    = "seshySessions"
	OtelTelemetryKey    = "otelTelemetry"
)

type document struct {
	Paths map[string]string `json:"paths"`
}

func manifestFile() (string, error) {
	if override := os.Getenv("SYSINIT_PATHS_MANIFEST"); override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf("SYSINIT_PATHS_MANIFEST must be absolute: %q", override)
		}
		return filepath.Clean(override), nil
	}
	state, err := StateHomeE()
	if err != nil {
		return "", err
	}
	return filepath.Join(state, "sysinit", "paths.json"), nil
}

func fallbackStateHome() string {
	if home, err := StateHomeE(); err == nil {
		return home
	}
	return filepath.Join(home(), ".local", "state")
}

func StateHomeE() (string, error) { return xdg.StateHome() }

func StateHome() string {
	return fallbackStateHome()
}

func ConfigHomeE() (string, error) { return xdg.ConfigHome() }

func ConfigHome() string {
	if root, err := ConfigHomeE(); err == nil {
		return root
	}
	return filepath.Join(home(), ".config")
}

func home() string {
	if dir := os.Getenv("HOME"); dir != "" {
		return dir
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return dir
}

func load() (map[string]string, error) {
	manifest, err := manifestFile()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(manifest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read paths manifest: %w", err)
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode paths manifest: %w", err)
	}
	return doc.Paths, nil
}

func GetE(key string) (string, bool, error) {
	values, err := load()
	if err != nil {
		return "", false, err
	}
	value, ok := values[key]
	if !ok || value == "" {
		return "", false, nil
	}
	if !filepath.IsAbs(value) {
		return "", false, fmt.Errorf("path %q must be absolute: %q", key, value)
	}
	return filepath.Clean(value), true, nil
}

func Get(key string) (string, bool) {
	value, ok, err := GetE(key)
	return value, ok && err == nil
}

func SeshySessions() string {
	if value, ok := Get(SeshySessionsKey); ok {
		return value
	}
	return filepath.Join(fallbackStateHome(), "seshy", "sessions")
}

func AgentPanes() string {
	if value, ok := Get(AgentPanesKey); ok {
		return value
	}
	return filepath.Join(fallbackStateHome(), "agents", "panes")
}

func AgentDiffNotes() string {
	if value, ok := Get(AgentDiffNotesKey); ok {
		return value
	}
	return filepath.Join(fallbackStateHome(), "agents", "diff-notes")
}

func AgentEdits() string {
	if value, ok := Get(AgentEditsKey); ok {
		return value
	}
	return filepath.Join(fallbackStateHome(), "agents", "edits")
}

func AgentWtrun() string {
	if value, ok := Get(AgentWtrunKey); ok {
		return value
	}
	return filepath.Join(fallbackStateHome(), "agents", "wtrun")
}

func AgentWorker() string {
	if value, ok := Get(AgentWorkerKey); ok {
		return value
	}
	return filepath.Join(fallbackStateHome(), "agents", "worker")
}

func AgentTranscripts() string {
	if value, ok := Get(AgentTranscriptsKey); ok {
		return value
	}
	return filepath.Join(fallbackStateHome(), "agents", "transcripts")
}

func AgentWorklog() string {
	if value, ok := Get(AgentWorklogKey); ok {
		return value
	}
	return filepath.Join(fallbackStateHome(), "agents", "worklog.jsonl")
}

func OtelTelemetry() string {
	if value, ok := Get(OtelTelemetryKey); ok {
		return value
	}
	return filepath.Join(fallbackStateHome(), "sysinit", "otel", "telemetry.jsonl")
}
