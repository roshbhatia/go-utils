package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInvokeStreamsBoundedFramesWithoutShell(t *testing.T) {
	manifest := helperManifest()
	input := json.RawMessage(`{"mode":"ok","value":"a; touch never"}`)
	var streamed []string
	invocation, err := Invoke(context.Background(), manifest, Request{
		RequestID: "request-1", Capability: "inspect", Operation: "show", Input: input,
	}, InvokeOptions{
		Environment: []string{"PATH=" + os.Getenv("PATH")},
		OnEvent: func(event Event) error {
			streamed = append(streamed, event.Message)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Result.Status != ResultOK || strings.Join(streamed, ",") != "working" {
		t.Fatalf("invocation = %+v, streamed = %v", invocation, streamed)
	}
	var output map[string]string
	if err := json.Unmarshal(invocation.Result.Output, &output); err != nil {
		t.Fatal(err)
	}
	if output["argument"] != "a; touch never" {
		t.Fatalf("argument = %q", output["argument"])
	}
}

func TestInvokeRejectsUnknownFrameFields(t *testing.T) {
	manifest := helperManifest()
	_, err := Invoke(context.Background(), manifest, Request{
		RequestID: "request-2", Capability: "inspect", Input: json.RawMessage(`{"mode":"unknown","value":"x"}`),
	}, InvokeOptions{})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v", err)
	}
}

func TestInvokeKillsTimedOutProvider(t *testing.T) {
	manifest := helperManifest()
	started := time.Now()
	_, err := Invoke(context.Background(), manifest, Request{
		RequestID: "request-3", Capability: "inspect", Input: json.RawMessage(`{"mode":"sleep","value":"x"}`),
	}, InvokeOptions{Timeout: 50 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("timed out provider was not killed promptly")
	}
}

func helperManifest() Manifest {
	return Manifest{
		Version: Version, Name: "helper", Description: "Test provider",
		Command: []string{os.Args[0], "-test.run=TestProviderHelperProcess", "--"},
		Actions: map[string]Action{
			"inspect": {
				Description: "Inspect test input",
				Argv:        []string{"{{ .Input.value }}"},
				Env:         map[string]string{"PROVIDER_MODE": "{{ .Input.mode }}", "GO_WANT_PROVIDER_HELPER": "1"},
			},
		},
	}
}

func TestProviderHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_PROVIDER_HELPER") != "1" {
		return
	}
	requestLine, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var request Request
	if err := json.Unmarshal(requestLine, &request); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	switch os.Getenv("PROVIDER_MODE") {
	case "sleep":
		time.Sleep(5 * time.Second)
	case "unknown":
		fmt.Printf("{\"version\":%q,\"kind\":\"result\",\"requestId\":%q,\"status\":\"ok\",\"extra\":true}\n", Version, request.RequestID)
	default:
		encoder := json.NewEncoder(os.Stdout)
		_ = encoder.Encode(Event{Version: Version, Kind: FrameEvent, RequestID: request.RequestID, Event: "progress", Message: "working"})
		argument := ""
		for index, value := range os.Args {
			if value == "--" && index+1 < len(os.Args) {
				argument = os.Args[index+1]
			}
		}
		output, _ := json.Marshal(map[string]string{"argument": argument})
		_ = encoder.Encode(Result{Version: Version, Kind: FrameResult, RequestID: request.RequestID, Status: ResultOK, Output: output})
	}
	os.Exit(0)
}
