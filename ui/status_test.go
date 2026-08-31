package ui

import "testing"

func TestStatuses(t *testing.T) {
	t.Parallel()

	for _, status := range []Status{StatusIdle, StatusWorking, StatusWaiting, StatusBlocked, StatusFailed, StatusDone} {
		if !status.Valid() {
			t.Fatalf("status %q is invalid", status)
		}
	}
	if Status("unknown").Valid() {
		t.Fatal("unknown status is valid")
	}
}

func TestSpinnerFramesReturnsCopy(t *testing.T) {
	t.Parallel()

	first := SpinnerFrames()
	first[0] = "changed"
	if SpinnerFrames()[0] == "changed" {
		t.Fatal("spinner frames share mutable state")
	}
}
