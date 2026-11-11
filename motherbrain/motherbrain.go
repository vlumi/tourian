// Package motherbrain is whoever is at the controls: the controller seam, and the arbiter that says which controller holds each robot.
package motherbrain

import (
	"context"
	"log"
	"sync"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/hub"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// A Controller inside the platform decides a robot's motion from each observation while it holds the robot. The operator is the other kind of controller and speaks through the hub instead.
type Controller interface {
	Describe() *tourianv1.Controller
	Act(ctx context.Context, observation *tourianv1.Observation) (*tourianv1.Twist, error)
}

// Platform is what the arbiter needs from the hub.
type Platform interface {
	Publish(message proto.Message)
	SendToRobot(robotID string, message proto.Message)
}

const (
	inboxDepth    = 1024
	actTimeout    = 40 * time.Millisecond
	complainEach  = time.Second
	operatorLease = time.Second
)

// The Arbiter holds one answer per robot to the question of who is driving. The operator takes over by driving and hands back by releasing; while one console's operator holds a robot, another console's driving is refused, until the holder falls silent for a lease and the robot is left in nobody's hands. A console hands a robot to a controller by name; an emergency stop from a console leaves the robot in nobody's hands, so no controller drives on after it. The holder's controller acts on every observation, one act in flight per robot, so a slow one holds up nobody else.
type Arbiter struct {
	platform    Platform
	now         func() time.Time
	controllers map[string]Controller
	available   []*tourianv1.Controller
	inbox       chan tapped
	mu          sync.Mutex
	robots      map[string]*holding
	sequence    uint64
	acting      sync.WaitGroup
}

type tapped struct {
	from hub.Source
	env  *tourianv1.Envelope
}

type holding struct {
	holder       *tourianv1.Controller
	previous     *tourianv1.Controller
	since        time.Time
	drivenAt     time.Time
	acting       bool
	complainedAt time.Time
}

func NewArbiter(platform Platform, now func() time.Time, controllers ...Controller) *Arbiter {
	a := &Arbiter{
		platform:    platform,
		now:         now,
		controllers: map[string]Controller{},
		inbox:       make(chan tapped, inboxDepth),
		robots:      map[string]*holding{},
	}
	for _, c := range controllers {
		description := c.Describe()
		a.controllers[description.GetId()] = c
		a.available = append(a.available, description)
	}
	return a
}

// Tap queues what the hub saw; a full inbox drops the message, and the robot's own timeouts cover a controller that falls silent.
func (a *Arbiter) Tap(from hub.Source, env *tourianv1.Envelope) {
	select {
	case a.inbox <- tapped{from, env}:
	default:
	}
}

// Run handles the inbox until ctx ends. Observations that piled up are skipped so a controller acts on the latest.
func (a *Arbiter) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-a.inbox:
			if item.env.GetObservation() != nil {
				item = a.latestObservation(item)
			}
			a.Handle(ctx, item.from, item.env)
		}
	}
}

func (a *Arbiter) latestObservation(item tapped) tapped {
	for {
		select {
		case next := <-a.inbox:
			if next.env.GetObservation() == nil || next.env.GetObservation().GetRobotId() != item.env.GetObservation().GetRobotId() {
				a.Handle(context.Background(), next.from, next.env)
				continue
			}
			item = next
		default:
			return item
		}
	}
}

// Handle applies one message; Run calls it from one goroutine, tests call it directly.
func (a *Arbiter) Handle(ctx context.Context, from hub.Source, env *tourianv1.Envelope) {
	switch m := env.GetMessage().(type) {
	case *tourianv1.Envelope_Capabilities:
		a.know(m.Capabilities.GetRobotId())
	case *tourianv1.Envelope_ControlRequest:
		if from.Kind == hub.SourceConsole {
			a.request(m.ControlRequest)
		}
	case *tourianv1.Envelope_Stop:
		if from.Kind == hub.SourceConsole {
			a.release(m.Stop.GetRobotId())
		}
	case *tourianv1.Envelope_Observation:
		a.act(ctx, m.Observation)
	}
}

// release takes the robot out of every hand: the operator's stop must not be undone by a controller's next command.
func (a *Arbiter) release(robotID string) {
	a.mu.Lock()
	h := a.robots[robotID]
	if h == nil || (h.holder == nil && h.previous == nil) {
		a.mu.Unlock()
		return
	}
	h.holder, h.previous = nil, nil
	h.since = a.now()
	a.mu.Unlock()
	a.publish(robotID)
}

func (a *Arbiter) know(robotID string) {
	a.mu.Lock()
	_, known := a.robots[robotID]
	if !known {
		a.robots[robotID] = &holding{since: a.now()}
	}
	a.mu.Unlock()
	if !known {
		a.publish(robotID)
	}
}

// Admit is the hub's question whether a console may drive the robot its message names: yes while that console's operator holds it, or nobody does, or a controller does, which the operator takes over from; no while another console's operator holds it and is still driving.
func (a *Arbiter) Admit(consoleID string, env *tourianv1.Envelope) bool {
	robotID := ws.RobotID(env)
	now := a.now()
	a.mu.Lock()
	h := a.robots[robotID]
	if h == nil {
		h = &holding{}
		a.robots[robotID] = h
	}
	switch {
	case operator(h.holder) && h.holder.GetId() != consoleID:
		if now.Sub(h.drivenAt) < operatorLease {
			a.mu.Unlock()
			return false
		}
		h.previous = nil
	case h.holder != nil && !operator(h.holder):
		h.previous = h.holder
	}
	takeover := h.holder.GetId() != consoleID
	if takeover {
		h.holder = &tourianv1.Controller{Id: consoleID, Kind: tourianv1.ControllerKind_CONTROLLER_KIND_OPERATOR, Name: "operator"}
		h.since = now
	}
	h.drivenAt = now
	a.mu.Unlock()
	if takeover {
		a.publish(robotID)
	}
	return true
}

func operator(c *tourianv1.Controller) bool {
	return c.GetKind() == tourianv1.ControllerKind_CONTROLLER_KIND_OPERATOR
}

// An operator that fell silent for a lease, by disconnecting or otherwise, leaves the robot in nobody's hands; the one held before is not resumed unasked.
func (a *Arbiter) expire(robotID string) {
	a.mu.Lock()
	h := a.robots[robotID]
	if h == nil || !operator(h.holder) || a.now().Sub(h.drivenAt) < operatorLease {
		a.mu.Unlock()
		return
	}
	h.holder, h.previous = nil, nil
	h.since = a.now()
	a.mu.Unlock()
	a.publish(robotID)
}

func (a *Arbiter) request(request *tourianv1.ControlRequest) {
	robotID := request.GetRobotId()
	a.mu.Lock()
	h := a.robots[robotID]
	if h == nil {
		a.mu.Unlock()
		return
	}
	switch {
	case request.GetRelease():
		h.holder, h.previous = h.previous, nil
	default:
		controller, ok := a.controllers[request.GetControllerId()]
		if !ok {
			a.mu.Unlock()
			return
		}
		h.holder, h.previous = controller.Describe(), nil
	}
	h.since = a.now()
	a.mu.Unlock()
	a.publish(robotID)
}

func (a *Arbiter) act(ctx context.Context, observation *tourianv1.Observation) {
	robotID := observation.GetRobotId()
	a.expire(robotID)
	a.mu.Lock()
	h := a.robots[robotID]
	var controller Controller
	if h != nil && h.holder != nil && !h.acting {
		controller = a.controllers[h.holder.GetId()]
	}
	if controller == nil {
		a.mu.Unlock()
		return
	}
	h.acting = true
	a.sequence++
	sequence := a.sequence
	a.mu.Unlock()

	a.acting.Add(1)
	go func() {
		defer a.acting.Done()
		ctx, cancel := context.WithTimeout(ctx, actTimeout)
		defer cancel()
		twist, err := controller.Act(ctx, observation)
		a.mu.Lock()
		h.acting = false
		stillHolds := h.holder != nil && h.holder.GetId() == controller.Describe().GetId()
		complain := err != nil && a.now().Sub(h.complainedAt) >= complainEach
		if complain {
			h.complainedAt = a.now()
		}
		a.mu.Unlock()
		if err != nil {
			if complain {
				log.Printf("motherbrain: %s on %s: %v", controller.Describe().GetId(), robotID, err)
			}
			return
		}
		if !stillHolds {
			return
		}
		header := &tourianv1.Header{Stamp: timestamppb.New(a.now()), FrameId: "base_link"}
		a.platform.SendToRobot(robotID, &tourianv1.Heartbeat{Header: header, RobotId: robotID, Sequence: sequence})
		a.platform.SendToRobot(robotID, &tourianv1.VelocityCommand{Header: header, RobotId: robotID, Sequence: sequence, Twist: twist})
	}()
}

// Wait blocks until every act in flight has finished, for tests and shutdown.
func (a *Arbiter) Wait() {
	a.acting.Wait()
}

// Holder reports who holds a robot, nil for nobody.
func (a *Arbiter) Holder(robotID string) *tourianv1.Controller {
	a.mu.Lock()
	defer a.mu.Unlock()
	if h := a.robots[robotID]; h != nil {
		return h.holder
	}
	return nil
}

func (a *Arbiter) publish(robotID string) {
	a.mu.Lock()
	h := a.robots[robotID]
	state := &tourianv1.ControlState{
		RobotId:   robotID,
		Holder:    h.holder,
		Since:     timestamppb.New(h.since),
		Available: a.available,
	}
	a.mu.Unlock()
	a.platform.Publish(state)
}
