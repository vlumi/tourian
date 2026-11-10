// Package contracttest is what every robot adapter must do, exercised through a real hub: announce itself, drive on command, stop when commands stop, stop when heartbeats stop, and stop at once on an emergency stop ahead of anything queued.
package contracttest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/hub"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// A Harness starts one adapter against the platform's robot endpoint and says which timeouts it was configured with.
type Harness struct {
	Start            func(ctx context.Context, platformRobotURL string)
	CommandTimeout   time.Duration
	HeartbeatTimeout time.Duration
}

const (
	commandInterval = 50 * time.Millisecond
	margin          = 400 * time.Millisecond
	speed           = 0.4
)

func Run(t *testing.T, h Harness) {
	t.Helper()
	hb := hub.New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /robot", hb.ServeRobot)
	mux.HandleFunc("GET /console", hb.ServeConsole)
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := "ws" + strings.TrimPrefix(server.URL, "http")

	go h.Start(ctx, base+"/robot")

	console, _, err := websocket.Dial(ctx, base+"/console", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = console.CloseNow() }()

	observations := make(chan *tourianv1.Observation, 256)
	capabilities := make(chan *tourianv1.Capabilities, 1)
	go func() {
		for {
			env, err := ws.Receive(ctx, console)
			if err != nil {
				return
			}
			switch m := env.GetMessage().(type) {
			case *tourianv1.Envelope_Capabilities:
				select {
				case capabilities <- m.Capabilities:
				default:
				}
			case *tourianv1.Envelope_Observation:
				select {
				case observations <- m.Observation:
				default:
				}
			}
		}
	}()

	var caps *tourianv1.Capabilities
	select {
	case caps = <-capabilities:
	case <-time.After(5 * time.Second):
		t.Fatal("the adapter should announce its capabilities")
	}
	if caps.GetRobotId() == "" || caps.GetCommandTimeout().AsDuration() != h.CommandTimeout || caps.GetHeartbeatTimeout().AsDuration() != h.HeartbeatTimeout {
		t.Fatalf("capabilities = %v", caps)
	}
	robotID := caps.GetRobotId()

	var heartbeats, commands atomic.Bool
	var sequence atomic.Uint64
	driver := func() {
		ticker := time.NewTicker(commandInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				seq := sequence.Add(1)
				header := &tourianv1.Header{Stamp: timestamppb.Now()}
				if heartbeats.Load() {
					_ = ws.Send(ctx, console, &tourianv1.Heartbeat{Header: header, RobotId: robotID, Sequence: seq})
				}
				if commands.Load() {
					_ = ws.Send(ctx, console, &tourianv1.VelocityCommand{Header: header, RobotId: robotID, Sequence: seq, Twist: &tourianv1.Twist{Linear: &tourianv1.Vector3{X: speed}, Angular: &tourianv1.Vector3{}}})
				}
			}
		}
	}
	go driver()

	await := func(what string, within time.Duration, ok func(*tourianv1.Observation) bool) {
		t.Helper()
		deadline := time.After(within)
		for {
			select {
			case obs := <-observations:
				if ok(obs) {
					return
				}
			case <-deadline:
				t.Fatalf("%s: not within %s", what, within)
			}
		}
	}
	moving := func(obs *tourianv1.Observation) bool {
		return obs.GetHaltReason() == tourianv1.HaltReason_HALT_REASON_UNSPECIFIED && obs.GetVelocity().GetLinear().GetX() > speed/2
	}
	haltedBy := func(reason tourianv1.HaltReason) func(*tourianv1.Observation) bool {
		return func(obs *tourianv1.Observation) bool {
			return obs.GetHaltReason() == reason && obs.GetVelocity().GetLinear().GetX() == 0
		}
	}

	t.Run("held still until the operator engages", func(t *testing.T) {
		await("dead man before any heartbeat", 2*time.Second, haltedBy(tourianv1.HaltReason_HALT_REASON_DEAD_MAN))
	})
	t.Run("drives on command", func(t *testing.T) {
		heartbeats.Store(true)
		commands.Store(true)
		await("moving", 2*time.Second, moving)
	})
	t.Run("stops when commands stop", func(t *testing.T) {
		commands.Store(false)
		await("command timeout", h.CommandTimeout+margin, haltedBy(tourianv1.HaltReason_HALT_REASON_COMMAND_TIMEOUT))
	})
	t.Run("stops when heartbeats stop", func(t *testing.T) {
		commands.Store(true)
		await("moving again", 2*time.Second, moving)
		heartbeats.Store(false)
		await("dead man", h.HeartbeatTimeout+margin, haltedBy(tourianv1.HaltReason_HALT_REASON_DEAD_MAN))
	})
	t.Run("emergency stop preempts the queue", func(t *testing.T) {
		heartbeats.Store(true)
		await("moving again", 2*time.Second, moving)
		commands.Store(false)
		for i := range 5 {
			header := &tourianv1.Header{Stamp: timestamppb.Now()}
			_ = ws.Send(ctx, console, &tourianv1.VelocityCommand{Header: header, RobotId: robotID, Sequence: 1_000_000 + uint64(i), Twist: &tourianv1.Twist{Linear: &tourianv1.Vector3{X: speed}, Angular: &tourianv1.Vector3{}}})
		}
		_ = ws.Send(ctx, console, &tourianv1.Stop{Header: &tourianv1.Header{Stamp: timestamppb.Now()}, RobotId: robotID})
		await("emergency stop", margin, haltedBy(tourianv1.HaltReason_HALT_REASON_EMERGENCY_STOP))
	})
}
