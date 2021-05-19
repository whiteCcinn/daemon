package daemon

type PipeMessageType int

const (
	SupervisorToProcess PipeMessageType = iota + 1
	ProcessToSupervisor
)

type ProcessBehavior int

const (
	WantSafetyClose ProcessBehavior = iota + 1
)

func (pmt PipeMessageType) String() (s string) {
	switch pmt {
	case SupervisorToProcess:
		s = "The Process sends messages to the Supervisor"
	case ProcessToSupervisor:
		s = "The Supervisor sends messages to the Process"
	default:
		s = "Unknown PipeMessageType"
	}
	return
}

func (pb ProcessBehavior) String() (s string) {
	switch pb {
	case WantSafetyClose:
		s = "Expect a safe exit"
	default:
		s = "Unknown ProcessBehavior"
	}
	return
}

type PipeMessage struct {
	Type     PipeMessageType
	Behavior ProcessBehavior
}
