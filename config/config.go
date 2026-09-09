// Package config loads typed CLI configuration from YAML and environment values.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/invopop/jsonschema"
	"github.com/roshbhatia/go-utils/xdg"
	"go.yaml.in/yaml/v3"
)

// Options names one application configuration surface.
type Options struct {
	Name      string
	EnvPrefix string
	Path      string
}

// Load applies YAML and environment values over defaults.
func Load[T any](defaults T, options Options) (T, error) {
	result := defaults
	path, err := resolvePath(options)
	if err != nil {
		return result, err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := decode(data, &result); err != nil {
			return result, fmt.Errorf("decode %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("read %s: %w", path, err)
	}
	if err := applyEnvironment(reflect.ValueOf(&result), options.EnvPrefix); err != nil {
		return result, err
	}
	return result, nil
}

// decode applies one YAML document to target. An empty, whitespace-only, or
// comment-only file holds no document and means no overrides, not a broken
// configuration.
func decode(data []byte, target any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// Path returns the selected YAML path without reading it.
func Path(options Options) (string, error) { return resolvePath(options) }

// Schema returns a Draft 2020-12 JSON Schema for T.
func Schema[T any](title string) ([]byte, error) {
	reflector := jsonschema.Reflector{Anonymous: true, ExpandedStruct: true}
	schema := reflector.Reflect(new(T))
	schema.Title = title
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode schema: %w", err)
	}
	return append(data, '\n'), nil
}

func resolvePath(options Options) (string, error) {
	if strings.TrimSpace(options.Name) == "" {
		return "", errors.New("config name is required")
	}
	if strings.TrimSpace(options.Name) != options.Name || options.Name == "." || options.Name == ".." ||
		filepath.IsAbs(options.Name) || filepath.Clean(options.Name) != options.Name || strings.ContainsAny(options.Name, `/\`) {
		return "", fmt.Errorf("config name must be one path component: %q", options.Name)
	}
	if options.Path != "" {
		return absolutePath("config path", options.Path)
	}
	prefix := strings.TrimSpace(options.EnvPrefix)
	if prefix != "" {
		if override := strings.TrimSpace(os.Getenv(prefix + "_CONFIG")); override != "" {
			return absolutePath(prefix+"_CONFIG", override)
		}
	}
	root, err := xdg.ConfigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, options.Name, "config.yaml"), nil
}

func absolutePath(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", name, err)
	}
	return filepath.Clean(absolute), nil
}

func applyEnvironment(target reflect.Value, prefix string) error {
	if prefix == "" {
		return nil
	}
	if target.Kind() != reflect.Pointer || target.IsNil() {
		return errors.New("config target must be a non-nil pointer")
	}
	return applyStruct(target.Elem(), strings.ToUpper(prefix))
}

func applyStruct(value reflect.Value, prefix string) error {
	if value.Kind() != reflect.Struct {
		return errors.New("config target must point to a struct")
	}
	typeOf := value.Type()
	for index := range value.NumField() {
		field := value.Field(index)
		definition := typeOf.Field(index)
		if !field.CanSet() || definition.Anonymous {
			continue
		}
		name := fieldName(definition)
		if name == "" {
			continue
		}
		environmentName := prefix + "_" + snake(name)
		if field.Kind() == reflect.Struct && field.Type() != reflect.TypeFor[time.Duration]() {
			if err := applyStruct(field, environmentName); err != nil {
				return err
			}
			continue
		}
		raw, ok := os.LookupEnv(environmentName)
		if !ok {
			continue
		}
		if field.Type() == reflect.TypeFor[time.Duration]() {
			duration, err := time.ParseDuration(raw)
			if err != nil {
				return fmt.Errorf("%s: %w", environmentName, err)
			}
			field.SetInt(int64(duration))
			continue
		}
		if err := yaml.Unmarshal([]byte(raw), field.Addr().Interface()); err != nil {
			return fmt.Errorf("%s: %w", environmentName, err)
		}
	}
	return nil
}

func fieldName(field reflect.StructField) string {
	for _, tagName := range []string{"json", "yaml"} {
		name := strings.Split(field.Tag.Get(tagName), ",")[0]
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
	}
	return field.Name
}

func snake(value string) string {
	var out strings.Builder
	for index, one := range value {
		if unicode.IsUpper(one) && index > 0 {
			out.WriteByte('_')
		}
		out.WriteRune(unicode.ToUpper(one))
	}
	return out.String()
}
