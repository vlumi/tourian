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
	case *tourianv1.CameraAnnounce:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_CameraAnnounce{CameraAnnounce: m}}, nil
	case *tourianv1.VideoRequest:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_VideoRequest{VideoRequest: m}}, nil
	case *tourianv1.SessionDescription:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_SessionDescription{SessionDescription: m}}, nil
	case *tourianv1.IceCandidate:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_IceCandidate{IceCandidate: m}}, nil
	case *tourianv1.VideoFrame:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_VideoFrame{VideoFrame: m}}, nil
	case *tourianv1.RecordRequest:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_RecordRequest{RecordRequest: m}}, nil
	case *tourianv1.EpisodeStatus:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_EpisodeStatus{EpisodeStatus: m}}, nil
	case *tourianv1.EpisodeFlag:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_EpisodeFlag{EpisodeFlag: m}}, nil
	case *tourianv1.ControlState:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_ControlState{ControlState: m}}, nil
	case *tourianv1.ControlRequest:
		return &tourianv1.Envelope{Message: &tourianv1.Envelope_ControlRequest{ControlRequest: m}}, nil
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
	case *tourianv1.Envelope_CameraAnnounce:
		return m.CameraAnnounce.GetRobotId()
	case *tourianv1.Envelope_VideoRequest:
		return m.VideoRequest.GetRobotId()
	case *tourianv1.Envelope_SessionDescription:
		return m.SessionDescription.GetRobotId()
	case *tourianv1.Envelope_IceCandidate:
		return m.IceCandidate.GetRobotId()
	case *tourianv1.Envelope_VideoFrame:
		return m.VideoFrame.GetRobotId()
	case *tourianv1.Envelope_RecordRequest:
		return m.RecordRequest.GetRobotId()
	case *tourianv1.Envelope_EpisodeStatus:
		return m.EpisodeStatus.GetRobotId()
	case *tourianv1.Envelope_ControlState:
		return m.ControlState.GetRobotId()
	case *tourianv1.Envelope_ControlRequest:
		return m.ControlRequest.GetRobotId()
	default:
		return ""
	}
}

// PeerID names the console a signaling message is for or from, empty for anything else.
func PeerID(env *tourianv1.Envelope) string {
	switch m := env.GetMessage().(type) {
	case *tourianv1.Envelope_VideoRequest:
		return m.VideoRequest.GetPeerId()
	case *tourianv1.Envelope_SessionDescription:
		return m.SessionDescription.GetPeerId()
	case *tourianv1.Envelope_IceCandidate:
		return m.IceCandidate.GetPeerId()
	default:
		return ""
	}
}

// SetPeerID stamps a signaling message with the console it comes from; other messages are left alone.
func SetPeerID(env *tourianv1.Envelope, id string) {
	switch m := env.GetMessage().(type) {
	case *tourianv1.Envelope_VideoRequest:
		m.VideoRequest.PeerId = id
	case *tourianv1.Envelope_SessionDescription:
		m.SessionDescription.PeerId = id
	case *tourianv1.Envelope_IceCandidate:
		m.IceCandidate.PeerId = id
	}
}
