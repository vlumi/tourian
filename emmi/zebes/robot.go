// Package zebes is the simulated robot: a differential drive in a square arena with wheel joints and a draining battery, stepped by a clock the caller controls.
package zebes

import (
	"math"
	"sync"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Config struct {
	RobotID           string
	Name              string
	MaxLinear         float64
	MaxAngular        float64
	WheelBase         float64
	WheelRadius       float64
	ArenaHalfWidth    float64
	CommandTimeout    time.Duration
	HeartbeatTimeout  time.Duration
	IdleDrainPerHour  float64
	DriveDrainPerHour float64
	QueueDepth        int
}

func DefaultConfig() Config {
	return Config{
		RobotID:           "zebes-1",
		Name:              "Zebes rover",
		MaxLinear:         1.0,
		MaxAngular:        math.Pi / 2,
		WheelBase:         0.4,
		WheelRadius:       0.1,
		ArenaHalfWidth:    5,
		CommandTimeout:    500 * time.Millisecond,
		HeartbeatTimeout:  time.Second,
		IdleDrainPerHour:  0.05,
		DriveDrainPerHour: 0.5,
		QueueDepth:        32,
	}
}

const frameID = "map"

var jointNames = []string{"wheel_left", "wheel_right"}

type Robot struct {
	mu  sync.Mutex
	cfg Config
	now func() time.Time

	x, y, theta           float64
	linear, angular       float64
	wheelLeft, wheelRight float64
	battery               float64

	queue           []*tourianv1.VelocityCommand
	lastCommandAt   time.Time
	lastHeartbeatAt time.Time
	lastSequence    uint64
	halt            tourianv1.HaltReason
}

// New places the robot at the origin, fully charged and held still until a heartbeat arrives.
func New(cfg Config, now func() time.Time) *Robot {
	return &Robot{cfg: cfg, now: now, battery: 1, halt: tourianv1.HaltReason_HALT_REASON_DEAD_MAN}
}

func (r *Robot) Capabilities() *tourianv1.Capabilities {
	return &tourianv1.Capabilities{
		RobotId:    r.cfg.RobotID,
		Name:       r.cfg.Name,
		JointNames: jointNames,
		VelocityLimits: &tourianv1.VelocityLimits{
			MaxLinear:  r.cfg.MaxLinear,
			MaxAngular: r.cfg.MaxAngular,
		},
		CommandTimeout:   durationpb.New(r.cfg.CommandTimeout),
		HeartbeatTimeout: durationpb.New(r.cfg.HeartbeatTimeout),
	}
}

// Command queues the command for the next step and acknowledges it at once.
func (r *Robot) Command(cmd *tourianv1.VelocityCommand) *tourianv1.CommandAck {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.queue) >= r.cfg.QueueDepth {
		r.queue = r.queue[1:]
	}
	r.queue = append(r.queue, cmd)
	return &tourianv1.CommandAck{
		Header:       r.header(),
		RobotId:      r.cfg.RobotID,
		Sequence:     cmd.GetSequence(),
		CommandStamp: cmd.GetHeader().GetStamp(),
	}
}

func (r *Robot) Heartbeat(*tourianv1.Heartbeat) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastHeartbeatAt = r.now()
}

// Stop halts the robot immediately and discards whatever was queued.
func (r *Robot) Stop(*tourianv1.Stop) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue = nil
	r.linear, r.angular = 0, 0
	r.halt = tourianv1.HaltReason_HALT_REASON_EMERGENCY_STOP
}

// Step applies the queued commands, enforces the timeouts and advances the world by dt.
func (r *Robot) Step(dt time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	seconds := dt.Seconds()

	for _, cmd := range r.queue {
		r.linear = clamp(cmd.GetTwist().GetLinear().GetX(), r.cfg.MaxLinear)
		r.angular = clamp(cmd.GetTwist().GetAngular().GetZ(), r.cfg.MaxAngular)
		r.lastSequence = cmd.GetSequence()
		r.lastCommandAt = now
		r.halt = tourianv1.HaltReason_HALT_REASON_UNSPECIFIED
	}
	r.queue = r.queue[:0]

	switch {
	case r.halt == tourianv1.HaltReason_HALT_REASON_EMERGENCY_STOP:
	case now.Sub(r.lastHeartbeatAt) > r.cfg.HeartbeatTimeout:
		r.halt = tourianv1.HaltReason_HALT_REASON_DEAD_MAN
	case now.Sub(r.lastCommandAt) > r.cfg.CommandTimeout:
		r.halt = tourianv1.HaltReason_HALT_REASON_COMMAND_TIMEOUT
	default:
		r.halt = tourianv1.HaltReason_HALT_REASON_UNSPECIFIED
	}
	if r.halt != tourianv1.HaltReason_HALT_REASON_UNSPECIFIED {
		r.linear, r.angular = 0, 0
	}

	r.theta = wrapAngle(r.theta + r.angular*seconds)
	r.x = clamp(r.x+r.linear*math.Cos(r.theta)*seconds, r.cfg.ArenaHalfWidth)
	r.y = clamp(r.y+r.linear*math.Sin(r.theta)*seconds, r.cfg.ArenaHalfWidth)

	left, right := r.wheelSpeeds()
	r.wheelLeft += left * seconds
	r.wheelRight += right * seconds

	effort := math.Abs(r.linear) / r.cfg.MaxLinear
	drain := (r.cfg.IdleDrainPerHour + effort*r.cfg.DriveDrainPerHour) * seconds / 3600
	r.battery = math.Max(0, r.battery-drain)
}

func (r *Robot) Observe() *tourianv1.Observation {
	r.mu.Lock()
	defer r.mu.Unlock()
	left, right := r.wheelSpeeds()
	return &tourianv1.Observation{
		Header:  r.header(),
		RobotId: r.cfg.RobotID,
		Pose: &tourianv1.Pose{
			Position:    &tourianv1.Point{X: r.x, Y: r.y},
			Orientation: &tourianv1.Quaternion{Z: math.Sin(r.theta / 2), W: math.Cos(r.theta / 2)},
		},
		Velocity: &tourianv1.Twist{
			Linear:  &tourianv1.Vector3{X: r.linear},
			Angular: &tourianv1.Vector3{Z: r.angular},
		},
		Joints: &tourianv1.JointState{
			Name:     jointNames,
			Position: []float64{r.wheelLeft, r.wheelRight},
			Velocity: []float64{left, right},
		},
		Battery:             &tourianv1.Battery{Percentage: r.battery, Voltage: 11 + r.battery},
		LastCommandSequence: r.lastSequence,
		HaltReason:          r.halt,
	}
}

func (r *Robot) wheelSpeeds() (left, right float64) {
	half := r.angular * r.cfg.WheelBase / 2
	return (r.linear - half) / r.cfg.WheelRadius, (r.linear + half) / r.cfg.WheelRadius
}

func (r *Robot) header() *tourianv1.Header {
	return &tourianv1.Header{Stamp: timestamppb.New(r.now()), FrameId: frameID}
}

func clamp(v, limit float64) float64 {
	return math.Max(-limit, math.Min(limit, v))
}

func wrapAngle(a float64) float64 {
	return math.Atan2(math.Sin(a), math.Cos(a))
}
