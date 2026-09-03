package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"go.yaml.in/yaml/v3"
)

// LoadedManifest retains the source path used for diagnostics.
type LoadedManifest struct {
	Manifest Manifest
	Path     string
}

// Discover loads JSON and YAML manifests in lexical path order.
func Discover(directory string) ([]LoadedManifest, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read provider directory %s: %w", directory, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	loaded := make([]LoadedManifest, 0, len(entries))
	seen := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		extension := filepath.Ext(entry.Name())
		if extension != ".json" && extension != ".yaml" && extension != ".yml" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read provider manifest %s: %w", path, err)
		}
		manifest, err := Decode(bytes.NewReader(data), extension)
		if err != nil {
			return nil, fmt.Errorf("decode provider manifest %s: %w", path, err)
		}
		if previous, exists := seen[manifest.Name]; exists {
			return nil, fmt.Errorf("duplicate provider %q in %s and %s", manifest.Name, previous, path)
		}
		seen[manifest.Name] = path
		loaded = append(loaded, LoadedManifest{Manifest: manifest, Path: path})
	}
	return loaded, nil
}

// Decode strictly decodes one JSON or YAML provider manifest.
func Decode(reader io.Reader, extension string) (Manifest, error) {
	var manifest Manifest
	switch extension {
	case ".json", "json":
		decoder := json.NewDecoder(reader)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&manifest); err != nil {
			return manifest, err
		}
		if err := ensureJSONEnd(decoder); err != nil {
			return manifest, err
		}
	case ".yaml", ".yml", "yaml", "yml":
		decoder := yaml.NewDecoder(reader)
		decoder.KnownFields(true)
		if err := decoder.Decode(&manifest); err != nil {
			return manifest, err
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			if err == nil {
				return manifest, errors.New("manifest must contain one YAML document")
			}
			return manifest, err
		}
	default:
		return manifest, fmt.Errorf("unsupported manifest extension %q", extension)
	}
	if err := manifest.Validate(); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("manifest must contain one JSON value")
		}
		return err
	}
	return nil
}
