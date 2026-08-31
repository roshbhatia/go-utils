package ui

type Action string

const (
	ActionUp         Action = "up"
	ActionDown       Action = "down"
	ActionLeft       Action = "left"
	ActionRight      Action = "right"
	ActionOpen       Action = "open"
	ActionBack       Action = "back"
	ActionQuit       Action = "quit"
	ActionToggleTree Action = "toggle-tree"
)

type Key struct {
	Name string
	Ctrl bool
}

func ActionFor(key Key) (Action, bool) {
	if key.Ctrl {
		switch key.Name {
		case "h":
			return ActionLeft, true
		case "j":
			return ActionDown, true
		case "k":
			return ActionUp, true
		case "l":
			return ActionRight, true
		}
	}

	switch key.Name {
	case "h", "left":
		return ActionLeft, true
	case "j", "down":
		return ActionDown, true
	case "k", "up":
		return ActionUp, true
	case "l", "right":
		return ActionRight, true
	case "enter":
		return ActionOpen, true
	case "esc":
		return ActionBack, true
	case "q":
		return ActionQuit, true
	case "e":
		return ActionToggleTree, true
	default:
		return "", false
	}
}
