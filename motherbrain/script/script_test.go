package script

import (
	"testing"
	"time"
)

func TestSquareAlternatesLegsAndTurns(t *testing.T) {
	start := time.Unix(1000, 0)
	if twist := TwistAt(start.Add(time.Second)); twist.GetLinear().GetX() != speed || twist.GetAngular().GetZ() != 0 {
		t.Errorf("first the leg: %v", twist)
	}
	if twist := TwistAt(start.Add(3 * time.Second)); twist.GetLinear().GetX() != 0 || twist.GetAngular().GetZ() != turnRate {
		t.Errorf("then the turn: %v", twist)
	}
	if twist := TwistAt(start.Add(5 * time.Second)); twist.GetLinear().GetX() != speed {
		t.Errorf("then a leg again: %v", twist)
	}
}
