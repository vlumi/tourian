package safety

import (
	"testing"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
)

func TestGuard(t *testing.T) {
	t0 := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	g := New(500*time.Millisecond, time.Second)
	if g.Halt(t0) != tourianv1.HaltReason_HALT_REASON_DEAD_MAN {
		t.Fatal("held until the first heartbeat")
	}
	g.Heartbeat(t0)
	if g.Halt(t0) != tourianv1.HaltReason_HALT_REASON_COMMAND_TIMEOUT {
		t.Fatal("held until the first command")
	}
	g.Command(t0)
	if g.Halt(t0.Add(400*time.Millisecond)) != tourianv1.HaltReason_HALT_REASON_UNSPECIFIED {
		t.Fatal("free to move")
	}
	if g.Halt(t0.Add(600*time.Millisecond)) != tourianv1.HaltReason_HALT_REASON_COMMAND_TIMEOUT {
		t.Fatal("commands stopped")
	}
	g.Command(t0.Add(600 * time.Millisecond))
	if g.Halt(t0.Add(1100*time.Millisecond)) != tourianv1.HaltReason_HALT_REASON_DEAD_MAN {
		t.Fatal("heartbeats stopped")
	}
	g.Heartbeat(t0.Add(1100 * time.Millisecond))
	g.Stop()
	if g.Halt(t0.Add(1100*time.Millisecond)) != tourianv1.HaltReason_HALT_REASON_EMERGENCY_STOP {
		t.Fatal("stopped")
	}
	g.Command(t0.Add(1100 * time.Millisecond))
	if g.Halt(t0.Add(1100*time.Millisecond)) != tourianv1.HaltReason_HALT_REASON_UNSPECIFIED {
		t.Fatal("a new command clears the stop")
	}
}
