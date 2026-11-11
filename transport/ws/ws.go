// Package ws carries contract messages over a WebSocket, one Envelope per binary frame.
package ws

import (
	"context"
	"fmt"

	"github.com/coder/websocket"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const maxFrameBytes = 1 << 20

// Send wraps a contract message in an Envelope and writes it as one frame.
func Send(ctx context.Context, c *websocket.Conn, message proto.Message) error {
	env, err := Wrap(message)
	if err != nil {
		return err
	}
	data, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageBinary, data)
}

// Receive reads one frame and decodes the Envelope in it.
func Receive(ctx context.Context, c *websocket.Conn) (*tourianv1.Envelope, error) {
	c.SetReadLimit(maxFrameBytes)
	kind, data, err := c.Read(ctx)
	if err != nil {
		return nil, err
	}
	if kind != websocket.MessageBinary {
		return nil, fmt.Errorf("ws: expected a binary frame, got %v", kind)
	}
	env := &tourianv1.Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		return nil, fmt.Errorf("ws: decoding envelope: %w", err)
	}
	return env, nil
}

var (
	envelopeOneof  = (&tourianv1.Envelope{}).ProtoReflect().Descriptor().Oneofs().ByName("message")
	envelopeFields = envelopeOneof.Fields()
)

// Wrap puts a contract message in an Envelope, which it finds a place for by type; an Envelope is returned as is.
func Wrap(message proto.Message) (*tourianv1.Envelope, error) {
	if env, ok := message.(*tourianv1.Envelope); ok {
		return env, nil
	}
	inner := message.ProtoReflect()
	for i := range envelopeFields.Len() {
		field := envelopeFields.Get(i)
		if field.Message().FullName() == inner.Descriptor().FullName() {
			env := &tourianv1.Envelope{}
			env.ProtoReflect().Set(field, protoreflect.ValueOfMessage(inner))
			return env, nil
		}
	}
	return nil, fmt.Errorf("ws: %T is not a contract message", message)
}

// RobotID names the robot a message is addressed to or comes from, empty when it has none.
func RobotID(env *tourianv1.Envelope) string {
	return stringField(env, "robot_id")
}

// PeerID names the console a signaling message is for or from, empty for anything else.
func PeerID(env *tourianv1.Envelope) string {
	return stringField(env, "peer_id")
}

// SetPeerID stamps a signaling message with the console it comes from; other messages are left alone.
func SetPeerID(env *tourianv1.Envelope, id string) {
	if inner, field := innerField(env, "peer_id"); field != nil {
		inner.Set(field, protoreflect.ValueOfString(id))
	}
}

func stringField(env *tourianv1.Envelope, name protoreflect.Name) string {
	if inner, field := innerField(env, name); field != nil {
		return inner.Get(field).String()
	}
	return ""
}

func innerField(env *tourianv1.Envelope, name protoreflect.Name) (protoreflect.Message, protoreflect.FieldDescriptor) {
	outer := env.ProtoReflect()
	set := outer.WhichOneof(envelopeOneof)
	if set == nil {
		return nil, nil
	}
	inner := outer.Get(set).Message()
	field := inner.Descriptor().Fields().ByName(name)
	if field == nil || field.Kind() != protoreflect.StringKind {
		return nil, nil
	}
	return inner, field
}
