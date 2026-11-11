package rosbridge

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/vlumi/tourian/emmi/contracttest"
	"github.com/vlumi/tourian/emmi/rosbridge/rosbridgetest"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func TestLeavesTheRobotStoppedOnTheWayOut(t *testing.T) {
	ros := rosbridgetest.New()
	defer ros.Close()
	hubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		ctx := r.Context()
		if _, err := ws.Receive(ctx, conn); err != nil {
			return
		}
		for seq := uint64(1); seq <= 20; seq++ {
			header := &tourianv1.Header{Stamp: timestamppb.Now()}
			_ = ws.Send(ctx, conn, &tourianv1.Heartbeat{Header: header, RobotId: "ros-1", Sequence: seq})
			_ = ws.Send(ctx, conn, &tourianv1.VelocityCommand{Header: header, RobotId: "ros-1", Sequence: seq, Twist: &tourianv1.Twist{Linear: &tourianv1.Vector3{X: 0.4}, Angular: &tourianv1.Vector3{}}})
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer hubServer.Close()

	cfg := DefaultConfig()
	cfg.RosbridgeURL = ros.URL()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- connect(ctx, "ws"+strings.TrimPrefix(hubServer.URL, "http"), cfg) }()
	deadline := time.After(3 * time.Second)
	for {
		if linear, _ := ros.Command(); linear == 0.4 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the robot should be driving")
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("connect should end with the cancellation, got %v", err)
	}
	deadline = time.After(time.Second)
	for {
		if linear, _ := ros.Command(); linear == 0 {
			return
		}
		select {
		case <-deadline:
			linear, _ := ros.Command()
			t.Fatalf("the robot should be left stopped, last twist %v", linear)
		case <-time.After(10 * time.Millisecond):
		}
	}
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
