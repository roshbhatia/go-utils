// Package keymap defines renderer-neutral key bindings and generated help.
package keymap

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type Binding struct {
	ID          string
	Keys        []string
	Display     string
	Short       string
	Description string
	Contexts    []string
	Hidden      bool
}

// Context is one active set; binding context facets are conjunctive requirements.
type Context []string

var ErrAmbiguous = errors.New("key is ambiguous in the active context")

type Hint struct {
	Keys  string
	Short string
}

func (hint Hint) String() string {
	return strings.TrimSpace(strings.TrimSpace(hint.Keys) + " " + strings.TrimSpace(hint.Short))
}

type HelpRow struct {
	ID          string
	Keys        string
	Description string
}

type Catalog struct {
	bindings []Binding
	byID     map[string]int
}

// New validates and copies bindings into an immutable catalog.
func New(bindings ...Binding) (Catalog, error) {
	copyOf := make([]Binding, len(bindings))
	for index, binding := range bindings {
		copyOf[index] = binding
		copyOf[index].Keys = append([]string(nil), binding.Keys...)
		copyOf[index].Contexts = append([]string(nil), binding.Contexts...)
	}
	catalog := Catalog{bindings: copyOf, byID: make(map[string]int, len(copyOf))}
	if err := catalog.Validate(); err != nil {
		return Catalog{}, err
	}
	for index, binding := range copyOf {
		catalog.byID[binding.ID] = index
	}
	return catalog, nil
}

// Must builds a static catalog or panics when its bindings are invalid.
func Must(bindings ...Binding) Catalog {
	catalog, err := New(bindings...)
	if err != nil {
		panic(err)
	}
	return catalog
}

func (catalog Catalog) Validate() error {
	seenIDs := map[string]struct{}{}
	for index, binding := range catalog.bindings {
		if strings.TrimSpace(binding.ID) == "" {
			return fmt.Errorf("binding %d: id is required", index)
		}
		if _, exists := seenIDs[binding.ID]; exists {
			return fmt.Errorf("binding %q: duplicate id", binding.ID)
		}
		seenIDs[binding.ID] = struct{}{}
		if len(binding.Keys) == 0 {
			return fmt.Errorf("binding %q: at least one key is required", binding.ID)
		}
		seenKeys := map[string]struct{}{}
		seenContexts := map[string]struct{}{}
		for _, context := range binding.Contexts {
			context = normalizeContext(context)
			if context == "" {
				return fmt.Errorf("binding %q: context is empty", binding.ID)
			}
			if _, exists := seenContexts[context]; exists {
				return fmt.Errorf("binding %q: duplicate context %q", binding.ID, context)
			}
			seenContexts[context] = struct{}{}
		}
		for _, key := range binding.Keys {
			key = normalizeKey(key)
			if key == "" {
				return fmt.Errorf("binding %q: key is empty", binding.ID)
			}
			if _, exists := seenKeys[key]; exists {
				return fmt.Errorf("binding %q: duplicate key %q", binding.ID, key)
			}
			seenKeys[key] = struct{}{}
		}
		if strings.TrimSpace(binding.Description) == "" {
			return fmt.Errorf("binding %q: description is required", binding.ID)
		}
	}
	return nil
}

// Bindings returns a deep copy in declaration order.
func (catalog Catalog) Bindings() []Binding {
	out := make([]Binding, len(catalog.bindings))
	for index, binding := range catalog.bindings {
		out[index] = cloneBinding(binding)
	}
	return out
}

func (catalog Catalog) Binding(id string) (Binding, bool) {
	index, ok := catalog.byID[id]
	if !ok {
		return Binding{}, false
	}
	return cloneBinding(catalog.bindings[index]), true
}

// Match requires every binding context facet and reports overlapping matches.
func (catalog Catalog) Match(key string, context Context) (Binding, bool, error) {
	key = normalizeKey(key)
	var match Binding
	found := false
	for _, binding := range catalog.bindings {
		if !active(binding.Contexts, context) {
			continue
		}
		for _, candidate := range binding.Keys {
			if normalizeKey(candidate) == key {
				if found {
					return Binding{}, false, fmt.Errorf("%w: %q matches %q and %q", ErrAmbiguous, key, match.ID, binding.ID)
				}
				match, found = binding, true
			}
		}
	}
	return cloneBinding(match), found, nil
}

func (catalog Catalog) Hint(id string) (Hint, error) {
	binding, ok := catalog.Binding(id)
	if !ok {
		return Hint{}, fmt.Errorf("unknown binding %q", id)
	}
	return Hint{Keys: displayKeys(binding), Short: binding.Short}, nil
}

func (catalog Catalog) Hints(ids ...string) ([]Hint, error) {
	hints := make([]Hint, 0, len(ids))
	for _, id := range ids {
		hint, err := catalog.Hint(id)
		if err != nil {
			return nil, err
		}
		hints = append(hints, hint)
	}
	return hints, nil
}

func (catalog Catalog) HintLine(separator string, ids ...string) (string, error) {
	hints, err := catalog.Hints(ids...)
	if err != nil {
		return "", err
	}
	values := make([]string, len(hints))
	for index, hint := range hints {
		values[index] = hint.String()
	}
	return strings.Join(values, separator), nil
}

// HelpRows applies the same conjunctive context rules as Match.
func (catalog Catalog) HelpRows(context Context) ([]HelpRow, error) {
	rows := []HelpRow{}
	activeKeys := map[string]string{}
	for _, binding := range catalog.bindings {
		if binding.Hidden || !active(binding.Contexts, context) {
			continue
		}
		for _, key := range binding.Keys {
			normalized := normalizeKey(key)
			if previous, exists := activeKeys[normalized]; exists {
				return nil, fmt.Errorf("%w: %q matches %q and %q", ErrAmbiguous, normalized, previous, binding.ID)
			}
			activeKeys[normalized] = binding.ID
		}
		rows = append(rows, HelpRow{
			ID:          binding.ID,
			Keys:        displayKeys(binding),
			Description: binding.Description,
		})
	}
	return rows, nil
}

func displayKeys(binding Binding) string {
	if value := strings.TrimSpace(binding.Display); value != "" {
		return value
	}
	return strings.Join(binding.Keys, " / ")
}

func active(bindingContexts []string, context Context) bool {
	if len(bindingContexts) == 0 {
		return true
	}
	for _, required := range bindingContexts {
		if !slices.ContainsFunc(context, func(candidate string) bool {
			return normalizeContext(candidate) == normalizeContext(required)
		}) {
			return false
		}
	}
	return true
}

func cloneBinding(binding Binding) Binding {
	binding.Keys = append([]string(nil), binding.Keys...)
	binding.Contexts = append([]string(nil), binding.Contexts...)
	return binding
}

func normalizeContext(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeKey(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "+")
	if len(parts) < 2 {
		return strings.TrimSpace(value)
	}
	for index := 0; index < len(parts)-1; index++ {
		modifier := strings.ToLower(strings.TrimSpace(parts[index]))
		switch modifier {
		case "ctrl", "control":
			parts[index] = "ctrl"
		case "alt", "option":
			parts[index] = "alt"
		case "shift":
			parts[index] = "shift"
		case "super", "cmd", "command", "meta":
			parts[index] = "super"
		default:
			parts[index] = strings.TrimSpace(parts[index])
		}
	}
	parts[len(parts)-1] = strings.TrimSpace(parts[len(parts)-1])
	return strings.Join(parts, "+")
}
