package animation

import (
	"os"
	"strings"
	"testing"
)

func loadFixture(t *testing.T) Config {
	t.Helper()
	source, err := os.ReadFile("../testdata/terminal-animation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	config, err := ParseYAML(source)
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func TestSelectsVariants(t *testing.T) {
	t.Parallel()
	config := loadFixture(t)
	full, ok := config.Select("loading", Preferences{})
	if !ok || full.Dimensions.Width != 4 {
		t.Fatal("full variant was not selected")
	}
	compact, _ := config.Select("loading", Preferences{Compact: true})
	if compact.Dimensions.Width != 1 {
		t.Fatal("compact variant was not selected")
	}
	reduced, _ := config.Select("loading", Preferences{Compact: true, ReducedMotion: true})
	if reduced.Frames[0].Content != "done" {
		t.Fatal("reduced motion must take precedence")
	}
}

func TestMatchesGoldenTiming(t *testing.T) {
	t.Parallel()
	config := loadFixture(t)
	sequence, _ := config.Select("loading", Preferences{})
	cases := []struct {
		elapsed  uint64
		index    int
		progress float64
	}{{0, 0, 0}, {99, 0, 0.9801}, {100, 1, 0}, {300, 1, 0}, {400, 0, 0}, {600, 2, 0}}
	for _, test := range cases {
		sample := sequence.Sample(test.elapsed)
		if sample.FrameIndex != test.index || abs(sample.Progress-test.progress) >= 0.000_001 {
			t.Errorf("elapsed %d: got %+v", test.elapsed, sample)
		}
	}
	compact, _ := config.Select("loading", Preferences{Compact: true})
	variableCases := []struct {
		elapsed  uint64
		index    int
		progress float64
	}{{0, 0, 0}, {79, 0, 0.9875}, {80, 1, 0}, {199, 1, 119.0 / 120.0}, {200, 0, 0}}
	for _, test := range variableCases {
		sample := compact.Sample(test.elapsed)
		if sample.FrameIndex != test.index || abs(sample.Progress-test.progress) >= 0.000_001 {
			t.Errorf("variable timing at %d: got %+v", test.elapsed, sample)
		}
	}
	reduced, _ := config.Select("loading", Preferences{ReducedMotion: true})
	if reduced.Sample(999).Progress != 0.999 || reduced.Sample(1_000).Progress != 1 {
		t.Fatal("once playback did not stop on its final frame")
	}
}

func TestPadsStableDisplayCells(t *testing.T) {
	t.Parallel()
	config := loadFixture(t)
	sequence, _ := config.Select("loading", Preferences{})
	rendered := sequence.Render(0)
	if rendered.Content != "a   \n    " || rendered.Width != 4 || rendered.Height != 2 {
		t.Fatalf("unexpected render: %#v", rendered)
	}
}

func TestRejectsMixedTiming(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("../testdata/terminal-animation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	invalid := strings.Replace(string(source), "        - content: a", "        - content: a\n          duration_ms: 100", 1)
	_, err = ParseYAML([]byte(invalid))
	if err == nil || !strings.Contains(err.Error(), "duration_ms is forbidden") {
		t.Fatalf("expected mixed timing error, got %v", err)
	}
}

func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
