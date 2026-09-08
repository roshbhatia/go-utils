// Package xdg resolves absolute XDG base directories.
package xdg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Roots struct {
	Config  string
	Cache   string
	Data    string
	State   string
	Runtime string
}

// Resolve ignores relative XDG values and always returns absolute base directories.
func Resolve() (Roots, error) {
	config, err := ConfigHome()
	if err != nil {
		return Roots{}, err
	}
	cache, err := CacheHome()
	if err != nil {
		return Roots{}, err
	}
	data, err := DataHome()
	if err != nil {
		return Roots{}, err
	}
	state, err := StateHome()
	if err != nil {
		return Roots{}, err
	}
	runtime, err := RuntimeDir()
	if err != nil {
		return Roots{}, err
	}
	return Roots{Config: config, Cache: cache, Data: data, State: state, Runtime: runtime}, nil
}

func ConfigHome() (string, error) {
	return homeRoot("XDG_CONFIG_HOME", ".config")
}

func CacheHome() (string, error) {
	return homeRoot("XDG_CACHE_HOME", ".cache")
}

func DataHome() (string, error) {
	return homeRoot("XDG_DATA_HOME", ".local", "share")
}

func StateHome() (string, error) {
	return homeRoot("XDG_STATE_HOME", ".local", "state")
}

func RuntimeDir() (string, error) {
	if value, ok := configuredRoot("XDG_RUNTIME_DIR"); ok {
		return value, nil
	}
	return "", nil
}

func homeRoot(environment string, fallback ...string) (string, error) {
	if value, ok := configuredRoot(environment); ok {
		return value, nil
	}
	home, err := absoluteHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{home}, fallback...)...), nil
}

func configuredRoot(environment string) (string, bool) {
	value := strings.TrimSpace(os.Getenv(environment))
	if value == "" || !filepath.IsAbs(value) {
		return "", false
	}
	return filepath.Clean(value), true
}

func absoluteHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user home directory: %w", err)
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("user home directory must be absolute: %q", home)
	}
	return filepath.Clean(home), nil
}
