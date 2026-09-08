package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type doc struct {
	Notes []map[string]any `json:"notes"`
}

func newStore(t *testing.T) *Store {
	t.Helper()
	return &Store{
		Path: filepath.Join(t.TempDir(), "sub", "store.json"),
		Validate: JSONValidator(func(d doc) error {
			if d.Notes == nil {
				return errors.New("notes must be an array")
			}
			return nil
		}),
		Initial: func() ([]byte, error) { return json.Marshal(doc{Notes: []map[string]any{}}) },
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadTreatsZeroByteAsAbsent(t *testing.T) {
	s := newStore(t)
	mustWrite(t, s.Path, nil)
	data, err := s.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := s.Validate(data); err != nil {
		t.Fatalf("initialized store did not validate: %v", err)
	}
}

func TestReadRefusesMalformed(t *testing.T) {
	s := newStore(t)
	mustWrite(t, s.Path, []byte("{not json"))
	if _, err := s.Read(); !errors.Is(err, ErrMalformed) {
		t.Fatalf("want ErrMalformed, got %v", err)
	}
}

func TestPublishRefusesMalformed(t *testing.T) {
	tests := []struct {
		name     string
		existing []byte
	}{
		{name: "absent"},
		{name: "present", existing: []byte(`{"notes":[{"keep":true}]}`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newStore(t)
			if tc.existing != nil {
				mustWrite(t, s.Path, tc.existing)
			}
			if err := s.Publish([]byte(`{"notes": "not an array"}`)); err == nil {
				t.Fatal("published a malformed document")
			}
			got, err := os.ReadFile(s.Path)
			if tc.existing == nil {
				if !os.IsNotExist(err) {
					t.Fatalf("a refused publish created the store: %v", err)
				}
				return
			}
			if err != nil || string(got) != string(tc.existing) {
				t.Fatalf("store after refused publish = %q, %v; want %q", got, err, tc.existing)
			}
		})
	}
}

func TestPublishWithoutValidatorAcceptsAnyBytes(t *testing.T) {
	s := &Store{Path: filepath.Join(t.TempDir(), "raw")}
	if err := s.Publish([]byte("anything\n")); err != nil {
		t.Fatalf("publish: %v", err)
	}
	got, err := s.Read()
	if err != nil || string(got) != "anything\n" {
		t.Fatalf("Read() = %q, %v", got, err)
	}
}

func TestPublishRefusesSymlink(t *testing.T) {
	s := newStore(t)
	dir := filepath.Dir(s.Path)
	target := filepath.Join(dir, "real.json")
	mustWrite(t, target, []byte(`{"notes":[]}`))
	if err := os.Symlink(target, s.Path); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish([]byte(`{"notes":[{"a":1}]}`)); !errors.Is(err, ErrSymlink) {
		t.Fatalf("want ErrSymlink, got %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"notes":[]}` {
		t.Fatalf("symlink target was modified: %s", got)
	}
}

func TestPublishLeavesNoTemporaryFile(t *testing.T) {
	s := newStore(t)
	if err := s.Publish([]byte(`{"notes":[]}`)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(s.Path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "store.json" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("directory after publish = %v", names)
	}
}

func TestLockSerializesWriters(t *testing.T) {
	s := newStore(t)
	const writers = 8
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := s.Lock()
			if err != nil {
				t.Errorf("lock: %v", err)
				return
			}
			defer release()
			data, err := s.Read()
			if err != nil {
				t.Errorf("read: %v", err)
				return
			}
			var d doc
			if err := json.Unmarshal(data, &d); err != nil {
				t.Errorf("unmarshal: %v", err)
				return
			}
			d.Notes = append(d.Notes, map[string]any{"n": 1})
			out, err := json.Marshal(d)
			if err != nil {
				t.Errorf("marshal: %v", err)
				return
			}
			if err := s.Publish(out); err != nil {
				t.Errorf("publish: %v", err)
			}
		}()
	}
	wg.Wait()

	data, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	var d doc
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Notes) != writers {
		t.Fatalf("lost a write: got %d notes, want %d", len(d.Notes), writers)
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	s := newStore(t)
	release, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	release()
	other, err := s.Lock()
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	release()
	if _, err := os.Stat(s.lockPath()); err != nil {
		t.Fatal("a stale release removed a lock it did not own")
	}
	other()
}

func TestWriteReplacesWholeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pane")
	mustWrite(t, path, []byte("old contents that are longer\n"))
	if err := Write(path, []byte("7\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "7\n" {
		t.Fatalf("file = %q, %v", got, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary file survived: %d entries", len(entries))
	}
}

func TestWriteDoesNotCreateDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "file")
	if err := Write(path, []byte("x")); err == nil {
		t.Fatal("Write created a directory it was not asked to create")
	}
}

func TestLeftoverTemporaryIsNotThePublishedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	// A writer that dies between CreateTemp and Rename leaves exactly this.
	stale, err := os.CreateTemp(filepath.Dir(path), tempPattern(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stale.WriteString("partial"); err != nil {
		t.Fatal(err)
	}
	_ = stale.Close()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("published path exists before any publish: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("a suffix glob saw the temporary file: %v", matches)
	}
	if !strings.HasPrefix(filepath.Base(stale.Name()), ".state.json.") {
		t.Fatalf("temporary name = %q", filepath.Base(stale.Name()))
	}

	s := &Store{Path: path}
	if err := s.Publish([]byte("whole\n")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "whole\n" {
		t.Fatalf("published = %q, %v", got, err)
	}
}

func TestSymlinkRetargetsWithoutRemovingPath(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "run-1.log")
	second := filepath.Join(dir, "run-2.log")
	mustWrite(t, first, []byte("1"))
	mustWrite(t, second, []byte("2"))
	link := filepath.Join(dir, "last.log")

	if err := Symlink(first, link); err != nil {
		t.Fatalf("first link: %v", err)
	}
	if err := Symlink(second, link); err != nil {
		t.Fatalf("second link: %v", err)
	}
	target, err := os.Readlink(link)
	if err != nil || target != second {
		t.Fatalf("Readlink = %q, %v; want %q", target, err, second)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("temporary link survived: %d entries", len(entries))
	}
}

func TestSymlinkDoesNotFollowExistingLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	mustWrite(t, target, []byte("keep"))
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other")
	mustWrite(t, other, []byte("other"))
	if err := Symlink(other, link); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "keep" {
		t.Fatalf("old target = %q, %v", got, err)
	}
}

func TestScrubbers(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		clean   string
		oneLine string
		control bool
	}{
		{name: "bell and escape", in: "a\x07b\nc\x1b[31md", clean: "ab\nc[31md", oneLine: "ab c[31md", control: true},
		{name: "unicode kept", in: "héllo → wörld\x07", clean: "héllo → wörld", oneLine: "héllo → wörld", control: true},
		{name: "newline only", in: "a\nb", clean: "a\nb", oneLine: "a b", control: true},
		{name: "plain path", in: "plain/path.txt", clean: "plain/path.txt", oneLine: "plain/path.txt", control: false},
		{name: "tab and cr", in: "a\tb\rc", clean: "abc", oneLine: "abc", control: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Clean(tc.in); got != tc.clean {
				t.Errorf("Clean = %q, want %q", got, tc.clean)
			}
			if got := OneLine(tc.in); got != tc.oneLine {
				t.Errorf("OneLine = %q, want %q", got, tc.oneLine)
			}
			if got := HasControlBytes(tc.in); got != tc.control {
				t.Errorf("HasControlBytes = %t, want %t", got, tc.control)
			}
		})
	}
}
