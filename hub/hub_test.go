package hub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/proto"
)

func TestRoutesBetweenRobotAndConsole(t *testing.T) {
	h := New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /robot", h.ServeRobot)
	mux.HandleFunc("GET /console", h.ServeConsole)
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	base := "ws" + strings.TrimPrefix(server.URL, "http")

	robot := dial(t, ctx, base+"/robot")
	send(t, ctx, robot, &tourianv1.Capabilities{RobotId: "r1", Name: "Robot one"})

	console := dial(t, ctx, base+"/console")
	if caps := receive(t, ctx, console).GetCapabilities(); caps.GetName() != "Robot one" {
		t.Fatalf("console should learn the robot on joining, got %v", caps)
	}

	send(t, ctx, console, &tourianv1.VelocityCommand{RobotId: "r1", Sequence: 1})
	if cmd := receive(t, ctx, robot).GetVelocityCommand(); cmd.GetSequence() != 1 {
		t.Fatalf("robot should receive the console's command, got %v", cmd)
	}

	send(t, ctx, console, &tourianv1.Stop{RobotId: "nobody"})
	send(t, ctx, robot, &tourianv1.Observation{RobotId: "r1", LastCommandSequence: 1})
	if obs := receive(t, ctx, console).GetObservation(); obs.GetLastCommandSequence() != 1 {
		t.Fatalf("console should receive the robot's observation, got %v", obs)
	}
}

func TestRobotMustAnnounceFirst(t *testing.T) {
	h := New()
	server := httptest.NewServer(http.HandlerFunc(h.ServeRobot))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	robot := dial(t, ctx, "ws"+strings.TrimPrefix(server.URL, "http"))
	send(t, ctx, robot, &tourianv1.Observation{RobotId: "r1"})
	if _, err := ws.Receive(ctx, robot); err == nil {
		t.Fatal("the hub should drop a robot whose first frame is not its capabilities")
	}
}

func dial(t *testing.T, ctx context.Context, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

func send(t *testing.T, ctx context.Context, conn *websocket.Conn, message proto.Message) {
	t.Helper()
	if err := ws.Send(ctx, conn, message); err != nil {
		t.Fatal(err)
	}
}

func receive(t *testing.T, ctx context.Context, conn *websocket.Conn) *tourianv1.Envelope {
	t.Helper()
	env, err := ws.Receive(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	return env
}
