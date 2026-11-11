package ws

import (
	"testing"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func FuzzEnvelope(f *testing.F) {
	for i := range envelopeFields.Len() {
		field := envelopeFields.Get(i)
		message := (&tourianv1.Envelope{}).ProtoReflect().NewField(field).Message()
		if robot := message.Descriptor().Fields().ByName("robot_id"); robot != nil {
			message.Set(robot, protoreflect.ValueOfString("r1"))
		}
		env, _ := Wrap(message.Interface())
		data, _ := proto.Marshal(env)
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		env := &tourianv1.Envelope{}
		if proto.Unmarshal(data, env) != nil {
			return
		}
		_ = RobotID(env)
		_ = PeerID(env)
		SetPeerID(env, "c1")
		if _, err := proto.Marshal(env); err != nil {
			t.Fatal(err)
		}
	})
}
