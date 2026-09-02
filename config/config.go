// Package config loads typed CLI configuration from YAML and environment values.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/invopop/jsonschema"
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
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&result); err != nil {
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
	if options.Name == "" {
		return "", errors.New("config name is required")
	}
	if options.Path != "" {
		return filepath.Clean(options.Path), nil
	}
	prefix := strings.TrimSpace(options.EnvPrefix)
	if prefix != "" {
		if override := strings.TrimSpace(os.Getenv(prefix + "_CONFIG")); override != "" {
			return filepath.Clean(override), nil
		}
	}
	root, err := configRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, options.Name, "config.yaml"), nil
}

func configRoot() (string, error) {
	if root := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); filepath.IsAbs(root) {
		return filepath.Clean(root), nil
	}
	if runtime.GOOS == "windows" {
		root, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("find user config directory: %w", err)
		}
		return root, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user home directory: %w", err)
	}
	return filepath.Join(home, ".config"), nil
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
