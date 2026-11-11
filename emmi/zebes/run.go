package zebes

import (
	"context"
	"log"
	"time"

	"github.com/coder/websocket"
	"github.com/vlumi/tourian/emmi"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/proto"
)

const (
	stepInterval    = 20 * time.Millisecond
	observeInterval = 50 * time.Millisecond
)

// Run keeps a robot connected to the platform until ctx ends, reconnecting whenever the connection drops. The world is reset on each connection.
func Run(ctx context.Context, platformURL string, cfg Config) error {
	return emmi.Reconnect(ctx, "zebes", func(ctx context.Context) error { return connect(ctx, platformURL, cfg) })
}

func connect(ctx context.Context, platformURL string, cfg Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, platformURL, nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.CloseNow() }()

	robot := New(cfg, time.Now)
	if err := ws.Send(ctx, conn, robot.Capabilities()); err != nil {
		return err
	}
	log.Printf("zebes: %q connected to %s", cfg.RobotID, platformURL)

	out := make(chan proto.Message, 64)
	failed := make(chan error, 2)
	go func() { failed <- readLoop(ctx, conn, robot, out) }()
	go func() { failed <- emmi.WriteLoop(ctx, conn, out) }()

	step := time.NewTicker(stepInterval)
	defer step.Stop()
	observe := time.NewTicker(observeInterval)
	defer observe.Stop()
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failed:
			return err
		case now := <-step.C:
			robot.Step(now.Sub(last))
			last = now
		case <-observe.C:
			select {
			case out <- robot.Observe():
			default:
			}
		}
	}
}

// readLoop applies what the platform sends as it arrives, so a stop takes effect before any command still queued for the next step.
func readLoop(ctx context.Context, conn *websocket.Conn, robot *Robot, out chan<- proto.Message) error {
	for {
		env, err := ws.Receive(ctx, conn)
		if err != nil {
			return err
		}
		switch m := env.GetMessage().(type) {
		case *tourianv1.Envelope_VelocityCommand:
			ack := robot.Command(m.VelocityCommand)
			select {
			case out <- ack:
			case <-ctx.Done():
				return ctx.Err()
			}
		case *tourianv1.Envelope_Heartbeat:
			robot.Heartbeat(m.Heartbeat)
		case *tourianv1.Envelope_Stop:
			robot.Stop(m.Stop)
		}
	}
}
