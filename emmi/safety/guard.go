// Package safety is the part of every robot adapter that must work with the console gone: a robot that has not heard a command within its timeout stops, one that has not heard a heartbeat stops, and an emergency stop holds until a new command arrives.
package safety

import (
	"sync"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
)

type Guard struct {
	mu               sync.Mutex
	commandTimeout   time.Duration
	heartbeatTimeout time.Duration
	lastCommandAt    time.Time
	lastHeartbeatAt  time.Time
	stopped          bool
}

// New makes a guard that holds the robot until the first heartbeat and command.
func New(commandTimeout, heartbeatTimeout time.Duration) *Guard {
	return &Guard{commandTimeout: commandTimeout, heartbeatTimeout: heartbeatTimeout}
}

// Command notes that a command was applied, which also clears an emergency stop.
func (g *Guard) Command(now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastCommandAt = now
	g.stopped = false
}

func (g *Guard) Heartbeat(now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastHeartbeatAt = now
}

func (g *Guard) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopped = true
}

// Halt says why the robot must be held still at this moment, or that it is free to move.
func (g *Guard) Halt(now time.Time) tourianv1.HaltReason {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case g.stopped:
		return tourianv1.HaltReason_HALT_REASON_EMERGENCY_STOP
	case now.Sub(g.lastHeartbeatAt) > g.heartbeatTimeout:
		return tourianv1.HaltReason_HALT_REASON_DEAD_MAN
	case now.Sub(g.lastCommandAt) > g.commandTimeout:
		return tourianv1.HaltReason_HALT_REASON_COMMAND_TIMEOUT
	default:
		return tourianv1.HaltReason_HALT_REASON_UNSPECIFIED
	}
}
