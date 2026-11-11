package ws

import (
	"testing"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestEveryContractMessageWrapsAndNamesItsRobot(t *testing.T) {
	for i := range envelopeFields.Len() {
		field := envelopeFields.Get(i)
		message := (&tourianv1.Envelope{}).ProtoReflect().NewField(field).Message()
		robot := message.Descriptor().Fields().ByName("robot_id")
		if robot != nil {
			message.Set(robot, protoreflect.ValueOfString("r1"))
		}
		env, err := Wrap(message.Interface())
		if err != nil {
			t.Fatalf("%s: %v", field.Name(), err)
		}
		if env.ProtoReflect().WhichOneof(envelopeOneof) != field {
			t.Fatalf("%s: wrapped under %v", field.Name(), env.ProtoReflect().WhichOneof(envelopeOneof))
		}
		if want := ""; robot != nil {
			want = "r1"
			if RobotID(env) != want {
				t.Fatalf("%s: robot id %q, want %q", field.Name(), RobotID(env), want)
			}
		}
	}
}

func TestSignalingCarriesThePeer(t *testing.T) {
	env, _ := Wrap(&tourianv1.SessionDescription{RobotId: "r1"})
	SetPeerID(env, "c1")
	if PeerID(env) != "c1" || env.GetSessionDescription().GetPeerId() != "c1" {
		t.Fatalf("peer id = %q", PeerID(env))
	}
	plain, _ := Wrap(&tourianv1.Stop{RobotId: "r1"})
	SetPeerID(plain, "c1")
	if PeerID(plain) != "" || !proto.Equal(plain.GetStop(), &tourianv1.Stop{RobotId: "r1"}) {
		t.Fatalf("a stop has no peer, got %v", plain)
	}
	if _, err := Wrap(&tourianv1.Twist{}); err == nil {
		t.Fatal("a twist is not a contract message on its own")
	}
}
