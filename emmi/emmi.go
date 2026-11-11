// Package emmi is the robot side: the adapters that speak the contract for a robot, and what they share.
package emmi

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/coder/websocket"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/proto"
)

const reconnectDelay = time.Second

// Reconnect runs connect until ctx ends, again after a delay whenever it returns, so an adapter outlives the platform and the robot.
func Reconnect(ctx context.Context, name string, connect func(ctx context.Context) error) error {
	for {
		err := connect(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("%s: %v; retrying in %s", name, err, reconnectDelay)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(reconnectDelay):
		}
	}
}

// WriteLoop sends what the adapter queues to the platform until ctx ends or a write fails.
func WriteLoop(ctx context.Context, conn *websocket.Conn, out <-chan proto.Message) error {
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

func Clamp(v, limit float64) float64 {
	return math.Max(-limit, math.Min(limit, v))
}
