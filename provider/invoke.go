package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"time"
)

const defaultTimeout = 30 * time.Second

// Limits bound provider output retained and parsed by an invocation.
type Limits struct {
	MaxLineBytes   int
	MaxOutputBytes int
	MaxStderrBytes int
	MaxEvents      int
}

func (limits Limits) withDefaults() Limits {
	if limits.MaxLineBytes <= 0 {
		limits.MaxLineBytes = 1024 * 1024
	}
	if limits.MaxOutputBytes <= 0 {
		limits.MaxOutputBytes = 4 * 1024 * 1024
	}
	if limits.MaxStderrBytes <= 0 {
		limits.MaxStderrBytes = 256 * 1024
	}
	if limits.MaxEvents <= 0 {
		limits.MaxEvents = 1000
	}
	return limits
}

// InvokeOptions controls one direct provider process.
type InvokeOptions struct {
	WorkingDirectory string
	Environment      []string
	TemplateData     any
	Timeout          time.Duration
	Limits           Limits
	Validator        Validator
	OnEvent          func(Event) error
}

// Invocation contains the provider result and its bounded telemetry.
type Invocation struct {
	Result     Result
	Events     []Event
	Stderr     string
	Duration   time.Duration
	Validation ValidationReport
}

// Invoke validates, renders, and directly executes one provider action.
func Invoke(
	ctx context.Context,
	manifest Manifest,
	request Request,
	options InvokeOptions,
) (Invocation, error) {
	started := time.Now()
	invocation := Invocation{}
	request, err := prepareRequest(request)
	if err != nil {
		return invocation, err
	}
	invocation.Validation = options.Validator.Validate(manifest, options.WorkingDirectory)
	if !invocation.Validation.OK() {
		return invocation, fmt.Errorf("provider %q is not valid: %w", manifest.Name, invocation.Validation.Error())
	}
	templateData := options.TemplateData
	if templateData == nil {
		templateData, err = defaultTemplateData(request, options.WorkingDirectory)
		if err != nil {
			return invocation, err
		}
	}
	plan, err := manifest.Render(request.Capability, templateData)
	if err != nil {
		return invocation, err
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = plan.Timeout
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	runContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	executable, err := lookupCommand(plan.Argv[0], options.WorkingDirectory, exec.LookPath, os.Stat)
	if err != nil {
		return invocation, fmt.Errorf("resolve provider executable: %w", err)
	}
	command := exec.CommandContext(runContext, executable, plan.Argv[1:]...)
	command.Dir = options.WorkingDirectory
	command.Env = mergedEnvironment(options.Environment, plan.Env)
	stdin, err := command.StdinPipe()
	if err != nil {
		return invocation, fmt.Errorf("open provider stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return invocation, fmt.Errorf("open provider stdout: %w", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return invocation, fmt.Errorf("open provider stderr: %w", err)
	}
	if err := command.Start(); err != nil {
		return invocation, fmt.Errorf("start provider %q: %w", manifest.Name, err)
	}
	limits := options.Limits.withDefaults()
	stderrBuffer := &boundedBuffer{limit: limits.MaxStderrBytes}
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(stderrBuffer, stderr)
		close(stderrDone)
	}()
	encodeError := json.NewEncoder(stdin).Encode(request)
	closeError := stdin.Close()
	if encodeError != nil {
		cancel()
		_ = command.Wait()
		<-stderrDone
		return invocation, fmt.Errorf("write provider request: %w", encodeError)
	}
	if closeError != nil {
		cancel()
		_ = command.Wait()
		<-stderrDone
		return invocation, fmt.Errorf("close provider stdin: %w", closeError)
	}

	parseError := readFrames(stdout, request.RequestID, limits, options.OnEvent, &invocation)
	if parseError != nil {
		cancel()
	}
	waitError := command.Wait()
	<-stderrDone
	invocation.Stderr = stderrBuffer.String()
	invocation.Duration = time.Since(started)
	if errors.Is(runContext.Err(), context.DeadlineExceeded) {
		return invocation, fmt.Errorf("provider %q timed out after %s: %w", manifest.Name, timeout, runContext.Err())
	}
	if parseError != nil {
		return invocation, fmt.Errorf("read provider %q output: %w", manifest.Name, parseError)
	}
	if waitError != nil {
		return invocation, fmt.Errorf("provider %q exited unsuccessfully: %w%s", manifest.Name, waitError, stderrSuffix(invocation.Stderr))
	}
	if invocation.Result.Kind == "" {
		return invocation, fmt.Errorf("provider %q emitted no result frame%s", manifest.Name, stderrSuffix(invocation.Stderr))
	}
	return invocation, nil
}

func defaultTemplateData(request Request, workingDirectory string) (map[string]any, error) {
	var input any
	if len(request.Input) > 0 {
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return nil, fmt.Errorf("decode request input for templates: %w", err)
		}
	}
	return map[string]any{
		"Capability":       request.Capability,
		"Context":          request.Context,
		"Input":            input,
		"Operation":        request.Operation,
		"RequestID":        request.RequestID,
		"Request":          request,
		"WorkingDirectory": workingDirectory,
	}, nil
}

func readFrames(
	reader io.Reader,
	requestID string,
	limits Limits,
	onEvent func(Event) error,
	invocation *Invocation,
) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, min(64*1024, limits.MaxLineBytes)), limits.MaxLineBytes)
	total := 0
	resultSeen := false
	for scanner.Scan() {
		line := scanner.Bytes()
		total += len(line) + 1
		if total > limits.MaxOutputBytes {
			return fmt.Errorf("output exceeds %d bytes", limits.MaxOutputBytes)
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if resultSeen {
			return errors.New("frame emitted after final result")
		}
		var header struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(line, &header); err != nil {
			return fmt.Errorf("decode frame header: %w", err)
		}
		switch header.Kind {
		case FrameEvent:
			var event Event
			if err := decodeStrictJSON(line, &event); err != nil {
				return fmt.Errorf("decode event: %w", err)
			}
			if err := validateEvent(event, requestID); err != nil {
				return err
			}
			if len(invocation.Events) >= limits.MaxEvents {
				return fmt.Errorf("event count exceeds %d", limits.MaxEvents)
			}
			invocation.Events = append(invocation.Events, event)
			if onEvent != nil {
				if err := onEvent(event); err != nil {
					return fmt.Errorf("handle event: %w", err)
				}
			}
		case FrameResult:
			if err := decodeStrictJSON(line, &invocation.Result); err != nil {
				return fmt.Errorf("decode result: %w", err)
			}
			if err := validateResult(invocation.Result, requestID); err != nil {
				return err
			}
			resultSeen = true
		default:
			return fmt.Errorf("unknown frame kind %q", header.Kind)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return ensureJSONEnd(decoder)
}

func mergedEnvironment(base []string, overrides map[string]string) []string {
	if base == nil {
		base = os.Environ()
	}
	values := make(map[string]string, len(base)+len(overrides))
	for _, value := range base {
		if index := bytes.IndexByte([]byte(value), '='); index >= 0 {
			values[value[:index]] = value[index+1:]
		}
	}
	for key, value := range overrides {
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func stderrSuffix(stderr string) string {
	if stderr == "" {
		return ""
	}
	return ": " + stderr
}

type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	original := len(data)
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = buffer.buffer.Write(data)
	}
	return original, nil
}

func (buffer *boundedBuffer) String() string { return buffer.buffer.String() }
