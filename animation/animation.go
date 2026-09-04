// Package animation implements the renderer-neutral terminal.animation/v1 contract.
package animation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/invopop/jsonschema"
	"go.yaml.in/yaml/v3"
)

const Version = "terminal.animation/v1"

type Playback string

const (
	PlaybackOnce     Playback = "once"
	PlaybackLoop     Playback = "loop"
	PlaybackPingPong Playback = "ping_pong"
)

type Easing string

const (
	EasingLinear    Easing = "linear"
	EasingEaseIn    Easing = "ease_in"
	EasingEaseOut   Easing = "ease_out"
	EasingEaseInOut Easing = "ease_in_out"
)

type Style string

const (
	StyleDefault Style = "default"
	StyleAccent  Style = "accent"
	StyleMuted   Style = "muted"
	StyleSuccess Style = "success"
	StyleWarning Style = "warning"
	StyleDanger  Style = "danger"
)

type Config struct {
	Version    string               `json:"version" yaml:"version" jsonschema:"enum=terminal.animation/v1"`
	Animations map[string]Animation `json:"animations" yaml:"animations" jsonschema:"minProperties=1"`
}

type Animation struct {
	Full          Sequence  `json:"full" yaml:"full"`
	Compact       *Sequence `json:"compact,omitempty" yaml:"compact,omitempty"`
	ReducedMotion Sequence  `json:"reduced_motion" yaml:"reduced_motion"`
}

type Sequence struct {
	Dimensions Dimensions `json:"dimensions" yaml:"dimensions"`
	Playback   Playback   `json:"playback" yaml:"playback" jsonschema:"enum=once,enum=loop,enum=ping_pong"`
	Easing     Easing     `json:"easing" yaml:"easing" jsonschema:"enum=linear,enum=ease_in,enum=ease_out,enum=ease_in_out"`
	FPS        *uint16    `json:"fps,omitempty" yaml:"fps,omitempty" jsonschema:"minimum=1,maximum=60"`
	Frames     []Frame    `json:"frames" yaml:"frames" jsonschema:"minItems=1,maxItems=512"`
}

type Dimensions struct {
	Width  uint16 `json:"width" yaml:"width" jsonschema:"minimum=1,maximum=512"`
	Height uint16 `json:"height" yaml:"height" jsonschema:"minimum=1,maximum=256"`
}

type Frame struct {
	Content    string  `json:"content" yaml:"content"`
	Style      Style   `json:"style" yaml:"style" jsonschema:"enum=default,enum=accent,enum=muted,enum=success,enum=warning,enum=danger"`
	DurationMS *uint64 `json:"duration_ms,omitempty" yaml:"duration_ms,omitempty" jsonschema:"minimum=1,maximum=10000"`
}

type Preferences struct {
	Compact       bool
	ReducedMotion bool
}

type Sample struct {
	FrameIndex int
	Progress   float64
}

type RenderedFrame struct {
	Content string
	Style   Style
	Width   uint16
	Height  uint16
}

func ParseYAML(source []byte) (Config, error) {
	var config Config
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode terminal animation: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple YAML documents are not supported")
		}
		return Config{}, fmt.Errorf("decode terminal animation: %w", err)
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) Validate() error {
	var problems []error
	if config.Version != Version {
		problems = append(problems, fmt.Errorf("version must equal %s", Version))
	}
	if len(config.Animations) == 0 {
		problems = append(problems, errors.New("animations must contain at least one animation"))
	}
	for name, animation := range config.Animations {
		if strings.TrimSpace(name) == "" {
			problems = append(problems, errors.New("animation names must not be empty"))
			continue
		}
		prefix := "animations." + name
		problems = appendIf(problems, animation.Full.validate(prefix+".full"))
		if animation.Compact != nil {
			problems = appendIf(problems, animation.Compact.validate(prefix+".compact"))
		}
		problems = appendIf(problems, animation.ReducedMotion.validate(prefix+".reduced_motion"))
		if len(animation.ReducedMotion.Frames) != 1 || animation.ReducedMotion.Playback != PlaybackOnce {
			problems = append(problems, fmt.Errorf("%s.reduced_motion must contain one frame with playback once", prefix))
		}
	}
	return errors.Join(problems...)
}

func (config Config) Select(name string, preferences Preferences) (*Sequence, bool) {
	animation, ok := config.Animations[name]
	if !ok {
		return nil, false
	}
	if preferences.ReducedMotion {
		return &animation.ReducedMotion, true
	}
	if preferences.Compact && animation.Compact != nil {
		return animation.Compact, true
	}
	return &animation.Full, true
}

func (sequence Sequence) Sample(elapsedMS uint64) Sample {
	path := sequence.playbackPath()
	durations := make([]uint64, len(path))
	var total uint64
	for index, frameIndex := range path {
		durations[index] = sequence.durationFor(frameIndex)
		total += durations[index]
	}
	position := elapsedMS
	stopped := sequence.Playback == PlaybackOnce && elapsedMS >= total
	if stopped {
		return Sample{FrameIndex: path[len(path)-1], Progress: 1}
	}
	if sequence.Playback != PlaybackOnce {
		position %= total
	}
	var start uint64
	for index, duration := range durations {
		end := start + duration
		if position < end {
			raw := float64(position-start) / float64(duration)
			return Sample{FrameIndex: path[index], Progress: sequence.ease(raw)}
		}
		start = end
	}
	panic("animation position exceeded validated duration")
}

func (sequence Sequence) Render(elapsedMS uint64) RenderedFrame {
	sample := sequence.Sample(elapsedMS)
	frame := sequence.Frames[sample.FrameIndex]
	lines := strings.Split(frame.Content, "\n")
	for index, line := range lines {
		lines[index] = line + strings.Repeat(" ", int(sequence.Dimensions.Width)-lipgloss.Width(line))
	}
	blank := strings.Repeat(" ", int(sequence.Dimensions.Width))
	for len(lines) < int(sequence.Dimensions.Height) {
		lines = append(lines, blank)
	}
	return RenderedFrame{
		Content: strings.Join(lines, "\n"),
		Style:   frame.Style,
		Width:   sequence.Dimensions.Width,
		Height:  sequence.Dimensions.Height,
	}
}

func Schema() ([]byte, error) {
	reflector := jsonschema.Reflector{Anonymous: true, ExpandedStruct: true}
	schema := reflector.Reflect(new(Config))
	schema.Title = "Renderer-neutral terminal animation"
	schema.ID = "https://roshbhatia.github.io/schemas/terminal.animation.v1.schema.json"
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode terminal animation schema: %w", err)
	}
	return append(data, '\n'), nil
}

func (sequence Sequence) validate(path string) error {
	var problems []error
	if sequence.Dimensions.Width < 1 || sequence.Dimensions.Width > 512 {
		problems = append(problems, fmt.Errorf("%s.dimensions.width must be between 1 and 512", path))
	}
	if sequence.Dimensions.Height < 1 || sequence.Dimensions.Height > 256 {
		problems = append(problems, fmt.Errorf("%s.dimensions.height must be between 1 and 256", path))
	}
	if len(sequence.Frames) < 1 || len(sequence.Frames) > 512 {
		problems = append(problems, fmt.Errorf("%s.frames must contain between 1 and 512 frames", path))
	}
	if !validPlayback(sequence.Playback) {
		problems = append(problems, fmt.Errorf("%s.playback is invalid", path))
	}
	if !validEasing(sequence.Easing) {
		problems = append(problems, fmt.Errorf("%s.easing is invalid", path))
	}
	if sequence.FPS != nil && (*sequence.FPS < 1 || *sequence.FPS > 60) {
		problems = append(problems, fmt.Errorf("%s.fps must be between 1 and 60", path))
	}
	for index, frame := range sequence.Frames {
		framePath := fmt.Sprintf("%s.frames[%d]", path, index)
		if !validStyle(frame.Style) {
			problems = append(problems, fmt.Errorf("%s.style is invalid", framePath))
		}
		switch {
		case sequence.FPS != nil && frame.DurationMS != nil:
			problems = append(problems, fmt.Errorf("%s.duration_ms is forbidden when fps is set", framePath))
		case sequence.FPS == nil && frame.DurationMS == nil:
			problems = append(problems, fmt.Errorf("%s.duration_ms is required when fps is absent", framePath))
		case frame.DurationMS != nil && (*frame.DurationMS < 1 || *frame.DurationMS > 10_000):
			problems = append(problems, fmt.Errorf("%s.duration_ms must be between 1 and 10000", framePath))
		}
		if strings.ContainsAny(frame.Content, "\r\t\x1b") {
			problems = append(problems, fmt.Errorf("%s.content must not contain carriage returns, tabs, or ANSI escapes", framePath))
		}
		lines := strings.Split(frame.Content, "\n")
		if len(lines) > int(sequence.Dimensions.Height) {
			problems = append(problems, fmt.Errorf("%s.content exceeds the declared height", framePath))
		}
		for _, line := range lines {
			if lipgloss.Width(line) > int(sequence.Dimensions.Width) {
				problems = append(problems, fmt.Errorf("%s.content exceeds the declared display-cell width", framePath))
				break
			}
		}
	}
	return errors.Join(problems...)
}

func (sequence Sequence) playbackPath() []int {
	path := make([]int, len(sequence.Frames))
	for index := range sequence.Frames {
		path[index] = index
	}
	if sequence.Playback == PlaybackPingPong && len(path) > 2 {
		for index := len(path) - 2; index > 0; index-- {
			path = append(path, index)
		}
	}
	return path
}

func (sequence Sequence) durationFor(frameIndex int) uint64 {
	if sequence.FPS != nil {
		return (1_000 + uint64(*sequence.FPS) - 1) / uint64(*sequence.FPS)
	}
	return *sequence.Frames[frameIndex].DurationMS
}

func (sequence Sequence) ease(progress float64) float64 {
	switch sequence.Easing {
	case EasingEaseIn:
		return progress * progress
	case EasingEaseOut:
		return 1 - (1-progress)*(1-progress)
	case EasingEaseInOut:
		if progress < 0.5 {
			return 2 * progress * progress
		}
		return 1 - ((-2*progress+2)*(-2*progress+2))/2
	default:
		return progress
	}
}

func validPlayback(value Playback) bool {
	return value == PlaybackOnce || value == PlaybackLoop || value == PlaybackPingPong
}

func validEasing(value Easing) bool {
	return value == EasingLinear || value == EasingEaseIn || value == EasingEaseOut || value == EasingEaseInOut
}

func validStyle(value Style) bool {
	return value == StyleDefault || value == StyleAccent || value == StyleMuted || value == StyleSuccess || value == StyleWarning || value == StyleDanger
}

func appendIf(problems []error, err error) []error {
	if err != nil {
		return append(problems, err)
	}
	return problems
}
