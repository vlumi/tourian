package rosbridge

import (
	"context"
	"testing"
	"time"

	"github.com/vlumi/tourian/emmi/contracttest"
	"github.com/vlumi/tourian/emmi/rosbridge/rosbridgetest"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
)

func command(linear float64) *tourianv1.VelocityCommand {
	return &tourianv1.VelocityCommand{Twist: &tourianv1.Twist{Linear: &tourianv1.Vector3{X: linear}, Angular: &tourianv1.Vector3{}}}
}

func TestContract(t *testing.T) {
	ros := rosbridgetest.New()
	defer ros.Close()
	cfg := DefaultConfig()
	cfg.RosbridgeURL = ros.URL()
	cfg.JointNames = []string{"wheel_left", "wheel_right"}
	cfg.CommandTimeout = 300 * time.Millisecond
	cfg.HeartbeatTimeout = 600 * time.Millisecond
	contracttest.Run(t, contracttest.Harness{
		Start:            func(ctx context.Context, url string) { _ = Run(ctx, url, cfg) },
		CommandTimeout:   cfg.CommandTimeout,
		HeartbeatTimeout: cfg.HeartbeatTimeout,
	})
}

func TestTellsTheRobotToStopWhenHalted(t *testing.T) {
	cfg := DefaultConfig()
	a := newAdapter(cfg, time.Now)
	now := time.Now()
	a.guard.Heartbeat(now)
	a.command(command(0.3))
	if out := a.tick(now); out.Linear.X != 0.3 {
		t.Fatalf("commanded twist should go out, got %v", out)
	}
	if out := a.tick(now.Add(cfg.CommandTimeout + time.Millisecond)); out.Linear.X != 0 {
		t.Fatalf("zero should go out after the command timeout, got %v", out)
	}
	a.command(command(0.3))
	a.stop()
	if out := a.tick(now.Add(cfg.CommandTimeout + time.Millisecond)); out.Linear.X != 0 {
		t.Fatalf("zero should go out after a stop, got %v", out)
	}
}
