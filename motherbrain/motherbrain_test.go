package motherbrain

import (
	"context"
	"errors"
	"testing"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/hub"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/proto"
)

type fakePlatform struct {
	published []*tourianv1.ControlState
	sent      []proto.Message
}

func (p *fakePlatform) Publish(message proto.Message) {
	p.published = append(p.published, message.(*tourianv1.ControlState))
}
func (p *fakePlatform) SendToRobot(_ string, message proto.Message) { p.sent = append(p.sent, message) }

type constant struct {
	id    string
	twist *tourianv1.Twist
	err   error
}

func (c constant) Describe() *tourianv1.Controller {
	return &tourianv1.Controller{Id: c.id, Kind: tourianv1.ControllerKind_CONTROLLER_KIND_SCRIPT, Name: c.id}
}
func (c constant) Act(context.Context, *tourianv1.Observation) (*tourianv1.Twist, error) {
	return c.twist, c.err
}

func handle(t *testing.T, a *Arbiter, from hub.Source, message proto.Message) {
	t.Helper()
	env, err := ws.Wrap(message)
	if err != nil {
		t.Fatal(err)
	}
	a.Handle(context.Background(), from, env)
}

func TestHandsOverAndBack(t *testing.T) {
	platform := &fakePlatform{}
	script := constant{id: "square", twist: &tourianv1.Twist{Linear: &tourianv1.Vector3{X: 0.5}}}
	a := NewArbiter(platform, time.Now, script)
	robot := hub.Source{Kind: hub.SourceRobot, ID: "r1"}
	console := hub.Source{Kind: hub.SourceConsole, ID: "c1"}

	handle(t, a, robot, &tourianv1.Capabilities{RobotId: "r1"})
	if a.Holder("r1") != nil || len(platform.published) != 1 || len(platform.published[0].GetAvailable()) != 1 {
		t.Fatalf("a new robot is held by nobody and offered the controllers, got %v", platform.published)
	}

	handle(t, a, robot, &tourianv1.Observation{RobotId: "r1"})
	if len(platform.sent) != 0 {
		t.Fatal("nobody holds the robot, nothing should be sent")
	}

	handle(t, a, console, &tourianv1.ControlRequest{RobotId: "r1", ControllerId: "square"})
	if a.Holder("r1").GetId() != "square" {
		t.Fatalf("holder = %v", a.Holder("r1"))
	}
	handle(t, a, robot, &tourianv1.Observation{RobotId: "r1"})
	if len(platform.sent) != 2 || platform.sent[1].(*tourianv1.VelocityCommand).GetTwist().GetLinear().GetX() != 0.5 {
		t.Fatalf("the script should drive on each observation, sent %v", platform.sent)
	}

	handle(t, a, console, &tourianv1.VelocityCommand{RobotId: "r1"})
	if h := a.Holder("r1"); h.GetKind() != tourianv1.ControllerKind_CONTROLLER_KIND_OPERATOR || h.GetId() != "c1" {
		t.Fatalf("the operator takes over by driving, holder = %v", h)
	}
	handle(t, a, console, &tourianv1.VelocityCommand{RobotId: "r1"})
	handle(t, a, robot, &tourianv1.Observation{RobotId: "r1"})
	if len(platform.sent) != 2 {
		t.Fatal("while the operator holds the robot, the script must not drive")
	}

	handle(t, a, console, &tourianv1.ControlRequest{RobotId: "r1", Release: true})
	if a.Holder("r1").GetId() != "square" {
		t.Fatalf("release should hand back to the script, holder = %v", a.Holder("r1"))
	}
	handle(t, a, console, &tourianv1.ControlRequest{RobotId: "r1", Release: true})
	if a.Holder("r1") != nil {
		t.Fatalf("releasing again leaves nobody, holder = %v", a.Holder("r1"))
	}
	if len(platform.published) != 5 {
		t.Errorf("every change of hands is published, got %d", len(platform.published))
	}
}

func TestAFailingControllerSendsNothing(t *testing.T) {
	platform := &fakePlatform{}
	a := NewArbiter(platform, time.Now, constant{id: "broken", err: errors.New("no server")})
	handle(t, a, hub.Source{Kind: hub.SourceRobot, ID: "r1"}, &tourianv1.Capabilities{RobotId: "r1"})
	handle(t, a, hub.Source{Kind: hub.SourceConsole, ID: "c1"}, &tourianv1.ControlRequest{RobotId: "r1", ControllerId: "broken"})
	handle(t, a, hub.Source{Kind: hub.SourceRobot, ID: "r1"}, &tourianv1.Observation{RobotId: "r1"})
	if len(platform.sent) != 0 {
		t.Fatalf("sent %v", platform.sent)
	}
}
