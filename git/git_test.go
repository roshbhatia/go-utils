package git

import (
	"strings"
	"testing"
)

func TestCleanEnvRemovesInheritedDiffDrivers(t *testing.T) {
	t.Setenv("GIT_EXTERNAL_DIFF", "git-difftool--helper")
	t.Setenv("GIT_DIFFTOOL_EXTCMD", "changes")
	for _, entry := range CleanEnv() {
		if strings.HasPrefix(entry, "GIT_EXTERNAL_DIFF=") || strings.HasPrefix(entry, "GIT_DIFFTOOL_EXTCMD=") {
			t.Fatalf("inherited diff driver survived: %s", entry)
		}
	}
}
