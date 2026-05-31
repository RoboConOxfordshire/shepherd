package run

import "fmt"

type State int

const (
	StateReady State = iota
	StateRunning
	StatePostRun
)

func (s State) String() string {
	switch s {
	case StateReady:
		return "ready"
	case StateRunning:
		return "running"
	case StatePostRun:
		return "post_run"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

type Mode string

const (
	ModeDevelopment Mode = "dev"
	ModeCompetition Mode = "comp"
)
