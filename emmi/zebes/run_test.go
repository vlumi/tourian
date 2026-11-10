package zebes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/hub"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDrivesThroughThePlatform(t *testing.T) {
	h := hub.New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /robot", h.ServeRobot)
	mux.HandleFunc("GET /console", h.ServeConsole)
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	base := "ws" + strings.TrimPrefix(server.URL, "http")

	go func() { _ = Run(ctx, base+"/robot", DefaultConfig()) }()

	console, _, err := websocket.Dial(ctx, base+"/console", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = console.CloseNow() }()

	var robotID string
	for robotID == "" {
		env, err := ws.Receive(ctx, console)
		if err != nil {
			t.Fatal(err)
		}
		robotID = env.GetCapabilities().GetRobotId()
	}

	acked := false
	moved := false
	ticker := time.NewTicker(stepInterval)
	defer ticker.Stop()
	deadline := time.After(3 * time.Second)
	received := make(chan *tourianv1.Envelope, 256)
	go func() {
		for {
			env, err := ws.Receive(ctx, console)
			if err != nil {
				return
			}
			received <- env
		}
	}()
	for seq := uint64(1); !acked || !moved; seq++ {
		select {
		case <-deadline:
			t.Fatalf("after 3 s: acked = %v, moved = %v", acked, moved)
		case env := <-received:
			switch m := env.GetMessage().(type) {
			case *tourianv1.Envelope_CommandAck:
				acked = acked || m.CommandAck.GetSequence() > 0
			case *tourianv1.Envelope_Observation:
				moved = moved || m.Observation.GetPose().GetPosition().GetX() > 0.05
			}
		case <-ticker.C:
			header := &tourianv1.Header{Stamp: timestamppb.Now()}
			if err := ws.Send(ctx, console, &tourianv1.Heartbeat{Header: header, RobotId: robotID, Sequence: seq}); err != nil {
				t.Fatal(err)
			}
			cmd := &tourianv1.VelocityCommand{
				Header:   header,
				RobotId:  robotID,
				Sequence: seq,
				Twist:    &tourianv1.Twist{Linear: &tourianv1.Vector3{X: 1}, Angular: &tourianv1.Vector3{}},
			}
			if err := ws.Send(ctx, console, cmd); err != nil {
				t.Fatal(err)
			}
		}
	}
}
