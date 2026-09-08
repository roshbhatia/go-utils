// Package store publishes small files atomically. Every writer lands its
// bytes in a hidden temporary file beside the target and renames it into
// place, so a reader sees the old file or the new file and never a partial
// one. A Store adds a lock directory, a validator, and fsync for state that
// must survive a crash; Write and Symlink serve cheap hook-side state that
// only needs to stay consistent.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type Validator func([]byte) error

var (
	ErrMalformed = errors.New("store is not valid")
	ErrSymlink   = errors.New("store is a symlink")
	ErrLockHeld  = errors.New("another process holds the store lock")
)

const (
	lockAttempts = 50
	lockInterval = 100 * time.Millisecond
)

// Store is a single file that other processes read while this one rewrites
// it. Validate runs on every Read and Publish; a nil Validate accepts any
// bytes. Initial supplies the document a missing or empty file stands for.
type Store struct {
	Path     string
	Validate Validator
	Initial  func() ([]byte, error)
}

func (s *Store) lockPath() string { return s.Path + ".lock" }

// Lock serializes writers with a directory beside the file, because mkdir is
// the one atomic create-or-fail every filesystem agrees on. The release is
// idempotent and never removes a lock it did not take.
func (s *Store) Lock() (release func(), err error) {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < lockAttempts; attempt++ {
		err := os.Mkdir(s.lockPath(), 0o755)
		if err == nil {
			var released bool
			return func() {
				if released {
					return
				}
				released = true
				_ = os.Remove(s.lockPath())
			}, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		time.Sleep(lockInterval)
	}
	return nil, fmt.Errorf("%w: %s", ErrLockHeld, s.lockPath())
}

// Read returns the validated document. A missing or zero-byte file yields
// Initial, so a first writer and a crashed writer look the same.
func (s *Store) Read() ([]byte, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(data) == 0 {
		if s.Initial == nil {
			return nil, ErrMalformed
		}
		return s.Initial()
	}
	if err := s.validate(data); err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrMalformed, s.Path, err)
	}
	return data, nil
}

// Publish validates data, refuses to replace a symlink, and writes through a
// synced temporary file. The file on disk is unchanged when Publish fails.
func (s *Store) Publish(data []byte) error {
	if err := s.validate(data); err != nil {
		return fmt.Errorf("refusing to publish a malformed store: %w", err)
	}
	if info, err := os.Lstat(s.Path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(s.Path)
		return fmt.Errorf("%w: %s -> %s", ErrSymlink, s.Path, target)
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	return write(s.Path, data, true)
}

func (s *Store) validate(data []byte) error {
	if s.Validate == nil {
		return nil
	}
	return s.Validate(data)
}

// Write replaces path with data through a temporary file in the same
// directory. It does not fsync and does not create the directory; a reader
// sees the previous bytes or the new bytes, and a power loss may lose the
// write. Rename replaces a symlink at path instead of following it.
func Write(path string, data []byte) error {
	return write(path, data, false)
}

func write(path string, data []byte, sync bool) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), tempPattern(path))
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if sync {
		if err := tmp.Sync(); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Symlink points path at target without a window where path is absent: the
// link is created under a temporary name and renamed over path.
func Symlink(target, path string) error {
	for attempt := 0; attempt < 3; attempt++ {
		name, err := tempName(path)
		if err != nil {
			return err
		}
		if err := os.Symlink(target, name); err != nil {
			if os.IsExist(err) {
				continue
			}
			return err
		}
		if err := os.Rename(name, path); err != nil {
			_ = os.Remove(name)
			return err
		}
		return nil
	}
	return fmt.Errorf("could not reserve a temporary link beside %s", path)
}

// A leading dot keeps a temporary file out of every suffix glob the readers
// of these directories use.
func tempPattern(path string) string {
	return "." + filepath.Base(path) + ".*"
}

func tempName(path string) (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	pattern := tempPattern(path)
	return filepath.Join(filepath.Dir(path), strings.TrimSuffix(pattern, "*")+hex.EncodeToString(raw[:])), nil
}

func JSONValidator[T any](check func(T) error) Validator {
	return func(data []byte) error {
		var doc T
		if err := json.Unmarshal(data, &doc); err != nil {
			return err
		}
		if check == nil {
			return nil
		}
		return check(doc)
	}
}

// Clean drops every control rune except newline.
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// OneLine folds newlines to spaces and drops every other control rune.
func OneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func HasControlBytes(s string) bool {
	return strings.ContainsFunc(s, unicode.IsControl)
}
