package orders

import "statebroker/sbis/internal/platform/httpx"

type Status string

const (
	Queued          Status = "QUEUED"
	New             Status = "NEW"
	Validating      Status = "VALIDATING"
	Validated       Status = "VALIDATED"
	Pending         Status = "PENDING"
	Routing         Status = "ROUTING"
	Sent            Status = "SENT"
	Acknowledged    Status = "ACKNOWLEDGED"
	PartiallyFilled Status = "PARTIALLY_FILLED"
	Filled          Status = "FILLED"
	CancelPending   Status = "CANCEL_PENDING"
	Cancelled       Status = "CANCELLED"
	Rejected        Status = "REJECTED"
	Expired         Status = "EXPIRED"
	Failed          Status = "FAILED"
)

var allowed = map[Status][]Status{
	Queued:          {Routing, Rejected, Failed},
	New:             {Validating, Rejected, Failed},
	Validating:      {Validated, Rejected, Failed},
	Validated:       {Pending, Rejected},
	Pending:         {Routing, Cancelled, Rejected, Failed},
	Routing:         {Sent, Failed, Rejected},
	Sent:            {Acknowledged, Rejected, Failed, CancelPending},
	Acknowledged:    {PartiallyFilled, Filled, CancelPending, Expired, Rejected},
	PartiallyFilled: {PartiallyFilled, Filled, CancelPending, Expired, Cancelled},
	CancelPending:   {Cancelled, PartiallyFilled, Filled, Rejected},
}

func CanTransition(from, to Status) bool {
	for _, n := range allowed[from] {
		if n == to {
			return true
		}
	}
	return false
}

func Transition(from, to Status) error {
	if !CanTransition(from, to) {
		return httpx.ErrInvalidTransition
	}
	return nil
}

func IsTerminal(s Status) bool {
	switch s {
	case Filled, Cancelled, Rejected, Expired, Failed:
		return true
	default:
		return false
	}
}
