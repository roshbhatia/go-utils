package ui

type Status string

const (
	StatusIdle    Status = "idle"
	StatusWorking Status = "working"
	StatusWaiting Status = "waiting"
	StatusBlocked Status = "blocked"
	StatusFailed  Status = "failed"
	StatusDone    Status = "done"
)

var statuses = map[Status]struct{}{
	StatusIdle: {}, StatusWorking: {}, StatusWaiting: {}, StatusBlocked: {}, StatusFailed: {}, StatusDone: {},
}

func (status Status) Valid() bool {
	_, ok := statuses[status]
	return ok
}

func SpinnerFrames() []string {
	return []string{"|", "/", "-", "\\"}
}
