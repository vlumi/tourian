// Package ws carries contract messages over a WebSocket, one Envelope per binary frame.
package ws

import (
	"context"
	"fmt"

	"github.com/coder/websocket"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/proto"
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

func Wrap(message proto.Message) (*tourianv1.Envelope, error) {
	switch m := message.(type) {
	case *tourianv1.Envelope:
		return m, nil
	case *tourianv1.Capabilities:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_Capabilities{Capabilities: m}}, nil
	case *tourianv1.VelocityCommand:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_VelocityCommand{VelocityCommand: m}}, nil
	case *tourianv1.CommandAck:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_CommandAck{CommandAck: m}}, nil
	case *tourianv1.Observation:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_Observation{Observation: m}}, nil
	case *tourianv1.Heartbeat:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_Heartbeat{Heartbeat: m}}, nil
	case *tourianv1.Stop:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_Stop{Stop: m}}, nil
	default:
		return nil, fmt.Errorf("ws: %T is not a contract message", message)
	}
}

// RobotID names the robot a message is addressed to or comes from, empty when it has none.
func RobotID(env *tourianv1.Envelope) string {
	switch m := env.GetMessage().(type) {
	case *tourianv1.Envelope_Capabilities:
		return m.Capabilities.GetRobotId()
	case *tourianv1.Envelope_VelocityCommand:
		return m.VelocityCommand.GetRobotId()
	case *tourianv1.Envelope_CommandAck:
		return m.CommandAck.GetRobotId()
	case *tourianv1.Envelope_Observation:
		return m.Observation.GetRobotId()
	case *tourianv1.Envelope_Heartbeat:
		return m.Heartbeat.GetRobotId()
	case *tourianv1.Envelope_Stop:
		return m.Stop.GetRobotId()
	default:
		return ""
	}
}
