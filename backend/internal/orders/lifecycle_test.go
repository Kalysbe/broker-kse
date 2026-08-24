package orders

import "testing"

func TestAllowedHappyPath(t *testing.T) {
	path := []Status{New, Validating, Validated, Pending, Routing, Sent, Acknowledged, Filled}
	for i := 0; i < len(path)-1; i++ {
		if err := Transition(path[i], path[i+1]); err != nil {
			t.Fatalf("%s -> %s: %v", path[i], path[i+1], err)
		}
	}
}

func TestRejectsArbitraryJump(t *testing.T) {
	if err := Transition(New, Filled); err == nil {
		t.Fatal("expected invalid transition")
	}
	if err := Transition(Filled, Cancelled); err == nil {
		t.Fatal("terminal must not move")
	}
}

func TestQueuedRoutesToFIX(t *testing.T) {
	if err := Transition(Queued, Routing); err != nil {
		t.Fatal(err)
	}
	if err := Transition(Queued, Filled); err == nil {
		t.Fatal("queued must not jump to filled")
	}
}

func TestPartialFillLoop(t *testing.T) {
	if err := Transition(PartiallyFilled, PartiallyFilled); err != nil {
		t.Fatal(err)
	}
	if err := Transition(PartiallyFilled, Filled); err != nil {
		t.Fatal(err)
	}
}
