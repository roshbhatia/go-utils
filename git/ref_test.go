package git

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckRefFormat(t *testing.T) {
	t.Parallel()

	if err := CheckRefFormat("feat/x"); err != nil {
		t.Fatalf("feat/x: %v", err)
	}
	for _, name := range []string{"HEAD", "-x", "a..b", "foo/", "foo.lock/bar", "foo//bar", "foo@{bar}", "a b", ""} {
		err := CheckRefFormat(name)
		var format *RefFormatError
		if !errors.As(err, &format) {
			t.Errorf("%q: %v, want *RefFormatError", name, err)
			continue
		}
		if format.Name != name || ExitStatus(err) != 128 || !strings.Contains(err.Error(), "not a valid branch name") {
			t.Errorf("%q: name %q exit %d message %q", name, format.Name, ExitStatus(err), err)
		}
	}
}
