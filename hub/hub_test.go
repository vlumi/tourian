package hub

import (
	"context"
	"fmt"
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
	h, base := serve(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	robot := dial(t, ctx, base+"/robot")
	send(t, ctx, robot, &tourianv1.Capabilities{RobotId: "r1", Name: "Robot one"})

	console := dial(t, ctx, base+"/console")
	welcome := receive(t, ctx, console).GetWelcome()
	if welcome.GetConsoleId() == "" {
		t.Fatalf("console should be told its id first, got %v", welcome)
	}
	if caps := receive(t, ctx, console).GetCapabilities(); caps.GetName() != "Robot one" {
		t.Fatalf("console should learn the robot on joining, got %v", caps)
	}
	h.Admit = func(consoleID string, env *tourianv1.Envelope) bool {
		return consoleID == welcome.GetConsoleId() && env.GetVelocityCommand().GetSequence() != 2
	}

	send(t, ctx, console, &tourianv1.VelocityCommand{RobotId: "r1", Sequence: 1})
	if cmd := receive(t, ctx, robot).GetVelocityCommand(); cmd.GetSequence() != 1 {
		t.Fatalf("robot should receive the console's command, got %v", cmd)
	}

	send(t, ctx, console, &tourianv1.VelocityCommand{RobotId: "r1", Sequence: 2})
	send(t, ctx, console, &tourianv1.Stop{RobotId: "nobody"})
	send(t, ctx, robot, &tourianv1.Observation{RobotId: "r1", LastCommandSequence: 1})
	if obs := receive(t, ctx, console).GetObservation(); obs.GetLastCommandSequence() != 1 {
		t.Fatalf("console should receive the robot's observation, got %v", obs)
	}
	send(t, ctx, console, &tourianv1.Stop{RobotId: "r1"})
	if receive(t, ctx, robot).GetStop() == nil {
		t.Fatal("the refused command should not have reached the robot, and a stop always does")
	}
}

func TestRelaysSignalingBetweenConsoleAndCamera(t *testing.T) {
	h, base := serve(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	camera := dial(t, ctx, base+"/camera")
	send(t, ctx, camera, &tourianv1.CameraAnnounce{RobotId: "r1", Kinds: []tourianv1.VideoKind{tourianv1.VideoKind_VIDEO_KIND_WEBRTC}})

	console := dial(t, ctx, base+"/console")
	receive(t, ctx, console)
	if announce := receive(t, ctx, console).GetCameraAnnounce(); announce.GetRobotId() != "r1" {
		t.Fatalf("console should learn the camera on joining, got %v", announce)
	}
	bystander := dial(t, ctx, base+"/console")
	for range 2 {
		receive(t, ctx, bystander)
	}

	send(t, ctx, console, &tourianv1.SessionDescription{RobotId: "r1", Type: "offer", Sdp: "v=0"})
	offer := receive(t, ctx, camera).GetSessionDescription()
	if offer.GetSdp() != "v=0" || offer.GetPeerId() == "" {
		t.Fatalf("camera should receive the offer stamped with the console's id, got %v", offer)
	}

	send(t, ctx, camera, &tourianv1.SessionDescription{RobotId: "r1", PeerId: offer.GetPeerId(), Type: "answer", Sdp: "v=1"})
	if answer := receive(t, ctx, console).GetSessionDescription(); answer.GetSdp() != "v=1" {
		t.Fatalf("the offering console should receive the answer, got %v", answer)
	}

	send(t, ctx, console, &tourianv1.VideoRequest{RobotId: "r1", Kind: tourianv1.VideoKind_VIDEO_KIND_MJPEG, Enabled: true})
	if request := receive(t, ctx, camera).GetVideoRequest(); !request.GetEnabled() || request.GetPeerId() != offer.GetPeerId() {
		t.Fatalf("camera should receive the request stamped with the console's id, got %v", request)
	}
	send(t, ctx, camera, &tourianv1.VideoFrame{RobotId: "r1", Sequence: 1})
	if frame := receive(t, ctx, console).GetVideoFrame(); frame.GetSequence() != 1 {
		t.Fatalf("the console that asked should receive the frame, got %v", frame)
	}
	h.Publish(&tourianv1.ControlState{RobotId: "r1"})
	if env := receive(t, ctx, bystander); env.GetControlState() == nil {
		t.Fatalf("a console that did not ask should not receive the frame, got %v", env)
	}

	_ = console.CloseNow()
	if request := receive(t, ctx, camera).GetVideoRequest(); request.GetEnabled() || request.GetPeerId() != offer.GetPeerId() {
		t.Fatalf("the camera should be told the console left, got %v", request)
	}
}

func TestTapsAndPublishing(t *testing.T) {
	h, base := serve(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	seen := make(chan string, 16)
	h.AddTap(func(from Source, env *tourianv1.Envelope) {
		seen <- fmt.Sprintf("%d:%T", from.Kind, env.GetMessage())
	})

	robot := dial(t, ctx, base+"/robot")
	send(t, ctx, robot, &tourianv1.Capabilities{RobotId: "r1"})
	if got := <-seen; got != "0:*tourianv1.Envelope_Capabilities" {
		t.Fatalf("tap saw %s", got)
	}

	h.Publish(&tourianv1.ControlState{RobotId: "r1", Holder: &tourianv1.Controller{Id: "script"}})
	if got := <-seen; got != "3:*tourianv1.Envelope_ControlState" {
		t.Fatalf("tap saw %s", got)
	}

	console := dial(t, ctx, base+"/console")
	receive(t, ctx, console)
	kinds := map[string]bool{}
	for range 2 {
		kinds[fmt.Sprintf("%T", receive(t, ctx, console).GetMessage())] = true
	}
	if !kinds["*tourianv1.Envelope_Capabilities"] || !kinds["*tourianv1.Envelope_ControlState"] {
		t.Fatalf("a late console should get the robot and the retained state, got %v", kinds)
	}

	h.SendToRobot("r1", &tourianv1.Stop{RobotId: "r1"})
	if got := <-seen; got != "3:*tourianv1.Envelope_Stop" {
		t.Fatalf("taps should see what the platform sends a robot, saw %s", got)
	}
	if receive(t, ctx, robot).GetStop() == nil {
		t.Fatal("robot should receive what the platform sends it")
	}
}

func TestRobotMustAnnounceFirst(t *testing.T) {
	_, base := serve(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	robot := dial(t, ctx, base+"/robot")
	send(t, ctx, robot, &tourianv1.Observation{RobotId: "r1"})
	if _, err := ws.Receive(ctx, robot); err == nil {
		t.Fatal("the hub should drop a robot whose first frame is not its capabilities")
	}
}

// serve starts a hub with its three endpoints and returns it with the ws base URL to dial.
func serve(t *testing.T) (*Hub, string) {
	t.Helper()
	h := New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /robot", h.ServeRobot)
	mux.HandleFunc("GET /camera", h.ServeCamera)
	mux.HandleFunc("GET /console", h.ServeConsole)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return h, "ws" + strings.TrimPrefix(server.URL, "http")
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

func TestBrowsersAreLetInByOrigin(t *testing.T) {
	h, base := serve(t)
	h.Origins = []string{"phone.local:*"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for origin, allowed := range map[string]bool{
		"http://localhost:1499":     true,
		"http://127.0.0.1:1499":     true,
		"http://phone.local:1499":   true,
		"http://elsewhere.example":  false,
		"http://phone.local.evil:1": false,
	} {
		conn, _, err := websocket.Dial(ctx, base+"/console", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {origin}}})
		if conn != nil {
			_ = conn.CloseNow()
		}
		if (err == nil) != allowed {
			t.Errorf("origin %s: allowed = %v, err = %v", origin, err == nil, err)
		}
		request := httptest.NewRequest(http.MethodGet, "http://"+strings.TrimPrefix(base, "ws://")+"/episodes", nil)
		if h.AllowsOrigin(request, origin) != allowed {
			t.Errorf("origin %s: AllowsOrigin = %v", origin, !allowed)
		}
	}
}

func TestAPeerThatNeverAnnouncesIsDropped(t *testing.T) {
	_, base := serve(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	robot := dial(t, ctx, base+"/robot")
	started := time.Now()
	if _, err := ws.Receive(ctx, robot); err == nil || time.Since(started) < announceTimeout/2 {
		t.Fatalf("the hub should close a silent robot after the announce timeout, got %v after %s", err, time.Since(started))
	}
}
