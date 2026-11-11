// Package rosbridge is the robot adapter for a ROS 2 robot reached through rosbridge: the contract on one side, ROS topics as JSON on the other, and the safety behaviors in between, so a robot that has not heard from the operator is told to stop whatever the console is doing.
package rosbridge

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/vlumi/tourian/emmi/safety"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Config struct {
	RobotID          string
	Name             string
	RosbridgeURL     string
	CmdVelTopic      string
	OdomTopic        string
	JointStatesTopic string
	BatteryTopic     string
	JointNames       []string
	MaxLinear        float64
	MaxAngular       float64
	CommandTimeout   time.Duration
	HeartbeatTimeout time.Duration
	HasCamera        bool
}

func DefaultConfig() Config {
	return Config{
		RobotID:          "ros-1",
		Name:             "ROS 2 robot",
		RosbridgeURL:     "ws://localhost:9090",
		CmdVelTopic:      "/cmd_vel",
		OdomTopic:        "/odom",
		JointStatesTopic: "/joint_states",
		BatteryTopic:     "/battery_state",
		MaxLinear:        0.5,
		MaxAngular:       1.0,
		CommandTimeout:   500 * time.Millisecond,
		HeartbeatTimeout: time.Second,
	}
}

const (
	publishInterval = 50 * time.Millisecond
	reconnectDelay  = time.Second
)

// Run keeps the adapter connected to the robot's rosbridge and to the platform until ctx ends, reconnecting whenever either side drops.
func Run(ctx context.Context, platformURL string, cfg Config) error {
	for {
		err := connect(ctx, platformURL, cfg)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("rosbridge: %q: %v; retrying in %s", cfg.RobotID, err, reconnectDelay)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(reconnectDelay):
		}
	}
}

func connect(ctx context.Context, platformURL string, cfg Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ros, _, err := websocket.Dial(ctx, cfg.RosbridgeURL, nil)
	if err != nil {
		return err
	}
	defer func() { _ = ros.CloseNow() }()
	defer leaveStopped(ros, cfg.CmdVelTopic)
	ros.SetReadLimit(1 << 20)
	platform, _, err := websocket.Dial(ctx, platformURL, nil)
	if err != nil {
		return err
	}
	defer func() { _ = platform.CloseNow() }()

	a := newAdapter(cfg, time.Now)
	if err := ws.Send(ctx, platform, a.capabilities()); err != nil {
		return err
	}
	for _, op := range []operation{
		{Op: "advertise", Topic: cfg.CmdVelTopic, Type: twistType},
		{Op: "subscribe", Topic: cfg.OdomTopic, Type: odometryType},
		{Op: "subscribe", Topic: cfg.JointStatesTopic, Type: jointStateType},
		{Op: "subscribe", Topic: cfg.BatteryTopic, Type: batteryType},
	} {
		if err := writeJSON(ctx, ros, op); err != nil {
			return err
		}
	}
	log.Printf("rosbridge: %q bridged %s to %s", cfg.RobotID, cfg.RosbridgeURL, platformURL)

	toPlatform := make(chan proto.Message, 64)
	failed := make(chan error, 3)
	go func() { failed <- a.readPlatform(ctx, platform, toPlatform) }()
	// The ROS side is read with a context of its own, so that cancelling ours does not close that socket before the last zero has gone out; closing it on the way out ends the read.
	go func() { failed <- a.readRos(context.Background(), ros, toPlatform) }()
	go func() { failed <- writePlatform(ctx, platform, toPlatform) }()

	ticker := time.NewTicker(publishInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failed:
			return err
		case now := <-ticker.C:
			msg, _ := json.Marshal(a.tick(now))
			if err := writeJSON(ctx, ros, operation{Op: "publish", Topic: cfg.CmdVelTopic, Msg: msg}); err != nil {
				return err
			}
		}
	}
}

// leaveStopped is the adapter's last word to the robot on the way out, whatever the reason: a plain ROS robot would otherwise keep the last twist it was told.
func leaveStopped(ros *websocket.Conn, topic string) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	msg, _ := json.Marshal(twist{})
	_ = writeJSON(ctx, ros, operation{Op: "publish", Topic: topic, Msg: msg})
}

type adapter struct {
	cfg   Config
	now   func() time.Time
	guard *safety.Guard

	mu           sync.Mutex
	commanded    twist
	lastSequence uint64
	halt         tourianv1.HaltReason
	joints       *tourianv1.JointState
	battery      *tourianv1.Battery
}

func newAdapter(cfg Config, now func() time.Time) *adapter {
	return &adapter{
		cfg:   cfg,
		now:   now,
		guard: safety.New(cfg.CommandTimeout, cfg.HeartbeatTimeout),
		halt:  tourianv1.HaltReason_HALT_REASON_DEAD_MAN,
	}
}

func (a *adapter) capabilities() *tourianv1.Capabilities {
	return &tourianv1.Capabilities{
		RobotId:          a.cfg.RobotID,
		Name:             a.cfg.Name,
		JointNames:       a.cfg.JointNames,
		VelocityLimits:   &tourianv1.VelocityLimits{MaxLinear: a.cfg.MaxLinear, MaxAngular: a.cfg.MaxAngular},
		CommandTimeout:   durationpb.New(a.cfg.CommandTimeout),
		HeartbeatTimeout: durationpb.New(a.cfg.HeartbeatTimeout),
		HasCamera:        a.cfg.HasCamera,
	}
}

func (a *adapter) command(cmd *tourianv1.VelocityCommand) *tourianv1.CommandAck {
	now := a.now()
	a.mu.Lock()
	a.commanded = twist{
		Linear:  vector3{X: clamp(cmd.GetTwist().GetLinear().GetX(), a.cfg.MaxLinear)},
		Angular: vector3{Z: clamp(cmd.GetTwist().GetAngular().GetZ(), a.cfg.MaxAngular)},
	}
	a.lastSequence = cmd.GetSequence()
	a.mu.Unlock()
	a.guard.Command(now)
	return &tourianv1.CommandAck{
		Header:       &tourianv1.Header{Stamp: timestamppb.New(now)},
		RobotId:      a.cfg.RobotID,
		Sequence:     cmd.GetSequence(),
		CommandStamp: cmd.GetHeader().GetStamp(),
	}
}

func (a *adapter) stop() {
	a.guard.Stop()
	a.mu.Lock()
	a.commanded = twist{}
	a.halt = tourianv1.HaltReason_HALT_REASON_EMERGENCY_STOP
	a.mu.Unlock()
}

// tick is what goes to /cmd_vel now: the commanded twist while the operator is heard, zero as soon as not. ROS robots are told every tick, since many stop on their own when the topic goes quiet and none should have to.
func (a *adapter) tick(now time.Time) twist {
	halt := a.guard.Halt(now)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.halt = halt
	if halt != tourianv1.HaltReason_HALT_REASON_UNSPECIFIED {
		return twist{}
	}
	return a.commanded
}

func (a *adapter) observation(odom odometry) *tourianv1.Observation {
	a.mu.Lock()
	defer a.mu.Unlock()
	at := odom.Header.Stamp.Time()
	if at.IsZero() {
		at = a.now()
	}
	pose := odom.Pose.Pose
	return &tourianv1.Observation{
		Header:  &tourianv1.Header{Stamp: timestamppb.New(at), FrameId: odom.Header.FrameID},
		RobotId: a.cfg.RobotID,
		Pose: &tourianv1.Pose{
			Position:    &tourianv1.Point{X: pose.Position.X, Y: pose.Position.Y, Z: pose.Position.Z},
			Orientation: &tourianv1.Quaternion{X: pose.Orientation.X, Y: pose.Orientation.Y, Z: pose.Orientation.Z, W: pose.Orientation.W},
		},
		Velocity: &tourianv1.Twist{
			Linear:  &tourianv1.Vector3{X: odom.Twist.Twist.Linear.X, Y: odom.Twist.Twist.Linear.Y, Z: odom.Twist.Twist.Linear.Z},
			Angular: &tourianv1.Vector3{X: odom.Twist.Twist.Angular.X, Y: odom.Twist.Twist.Angular.Y, Z: odom.Twist.Twist.Angular.Z},
		},
		Joints:              a.joints,
		Battery:             a.battery,
		LastCommandSequence: a.lastSequence,
		HaltReason:          a.halt,
	}
}

func (a *adapter) readPlatform(ctx context.Context, conn *websocket.Conn, out chan<- proto.Message) error {
	for {
		env, err := ws.Receive(ctx, conn)
		if err != nil {
			return err
		}
		switch m := env.GetMessage().(type) {
		case *tourianv1.Envelope_VelocityCommand:
			select {
			case out <- a.command(m.VelocityCommand):
			case <-ctx.Done():
				return ctx.Err()
			}
		case *tourianv1.Envelope_Heartbeat:
			a.guard.Heartbeat(a.now())
		case *tourianv1.Envelope_Stop:
			a.stop()
		}
	}
}

func (a *adapter) readRos(ctx context.Context, conn *websocket.Conn, out chan<- proto.Message) error {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var op operation
		if err := json.Unmarshal(data, &op); err != nil || op.Op != "publish" {
			continue
		}
		switch op.Topic {
		case a.cfg.OdomTopic:
			var odom odometry
			if json.Unmarshal(op.Msg, &odom) == nil {
				select {
				case out <- a.observation(odom):
				default:
				}
			}
		case a.cfg.JointStatesTopic:
			var joints jointState
			if json.Unmarshal(op.Msg, &joints) == nil {
				a.mu.Lock()
				a.joints = &tourianv1.JointState{Name: joints.Name, Position: joints.Position, Velocity: joints.Velocity, Effort: joints.Effort}
				a.mu.Unlock()
			}
		case a.cfg.BatteryTopic:
			var battery batteryState
			if json.Unmarshal(op.Msg, &battery) == nil {
				a.mu.Lock()
				a.battery = &tourianv1.Battery{Percentage: battery.Percentage, Voltage: battery.Voltage}
				a.mu.Unlock()
			}
		}
	}
}

func writePlatform(ctx context.Context, conn *websocket.Conn, out <-chan proto.Message) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message := <-out:
			if err := ws.Send(ctx, conn, message); err != nil {
				return err
			}
		}
	}
}

func writeJSON(ctx context.Context, conn *websocket.Conn, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func clamp(v, limit float64) float64 {
	return math.Max(-limit, math.Min(limit, v))
}
