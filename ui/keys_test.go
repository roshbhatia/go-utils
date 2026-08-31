package ui

import "testing"

func TestActionFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key    Key
		action Action
	}{
		{key: Key{Name: "j"}, action: ActionDown},
		{key: Key{Name: "up"}, action: ActionUp},
		{key: Key{Name: "h", Ctrl: true}, action: ActionLeft},
		{key: Key{Name: "enter"}, action: ActionOpen},
	}
	for _, test := range tests {
		action, ok := ActionFor(test.key)
		if !ok || action != test.action {
			t.Fatalf("ActionFor(%+v) = %q, %t; want %q, true", test.key, action, ok, test.action)
		}
	}
}
