package zebes

import (
	"math"
	"testing"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
)

const dt = 10 * time.Millisecond

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTestRobot() (*Robot, *clock) {
	c := &clock{t: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
	return New(DefaultConfig(), c.now), c
}

func drive(r *Robot, c *clock, linear, angular float64, d time.Duration, heartbeat bool) {
	for elapsed := time.Duration(0); elapsed < d; elapsed += dt {
		if heartbeat {
			r.Heartbeat(&tourianv1.Heartbeat{})
		}
		r.Command(command(linear, angular))
		c.t = c.t.Add(dt)
		r.Step(dt)
	}
}

func command(linear, angular float64) *tourianv1.VelocityCommand {
	return &tourianv1.VelocityCommand{
		Twist: &tourianv1.Twist{Linear: &tourianv1.Vector3{X: linear}, Angular: &tourianv1.Vector3{Z: angular}},
	}
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.02 {
		t.Errorf("%s = %.3f, want %.3f", name, got, want)
	}
}

func TestDrivesStraightAndTurns(t *testing.T) {
	r, c := newTestRobot()
	drive(r, c, 0.5, 0, time.Second, true)
	obs := r.Observe()
	near(t, "x", obs.GetPose().GetPosition().GetX(), 0.5)
	near(t, "y", obs.GetPose().GetPosition().GetY(), 0)
	near(t, "linear", obs.GetVelocity().GetLinear().GetX(), 0.5)

	drive(r, c, 0, math.Pi/2, time.Second, true)
	obs = r.Observe()
	near(t, "heading", 2*math.Atan2(obs.GetPose().GetOrientation().GetZ(), obs.GetPose().GetOrientation().GetW()), math.Pi/2)
	if obs.GetHaltReason() != tourianv1.HaltReason_HALT_REASON_UNSPECIFIED {
		t.Errorf("halt = %v while driving", obs.GetHaltReason())
	}
}

func TestClampsToLimitsAndArena(t *testing.T) {
	r, c := newTestRobot()
	drive(r, c, 10, 0, 20*time.Second, true)
	obs := r.Observe()
	near(t, "linear", obs.GetVelocity().GetLinear().GetX(), r.cfg.MaxLinear)
	near(t, "x", obs.GetPose().GetPosition().GetX(), r.cfg.ArenaHalfWidth)
}

func TestWheelsTurnWithTheDrive(t *testing.T) {
	r, c := newTestRobot()
	drive(r, c, 0, 1, time.Second, true)
	joints := r.Observe().GetJoints()
	if joints.GetVelocity()[0] >= 0 || joints.GetVelocity()[1] <= 0 {
		t.Errorf("turning left should run the left wheel backward and the right forward, got %v", joints.GetVelocity())
	}
	near(t, "left angle", joints.GetPosition()[0], -r.cfg.WheelBase/2/r.cfg.WheelRadius)
}

func TestStopsWhenCommandsStop(t *testing.T) {
	r, c := newTestRobot()
	drive(r, c, 1, 0, time.Second, true)
	for elapsed := time.Duration(0); elapsed <= r.cfg.CommandTimeout; elapsed += dt {
		r.Heartbeat(&tourianv1.Heartbeat{})
		c.t = c.t.Add(dt)
		r.Step(dt)
		if elapsed < r.cfg.CommandTimeout-dt && r.Observe().GetVelocity().GetLinear().GetX() == 0 {
			t.Fatalf("stopped after %v, before the command timeout", elapsed)
		}
	}
	obs := r.Observe()
	if obs.GetVelocity().GetLinear().GetX() != 0 || obs.GetHaltReason() != tourianv1.HaltReason_HALT_REASON_COMMAND_TIMEOUT {
		t.Errorf("after the timeout: linear = %v, halt = %v", obs.GetVelocity().GetLinear().GetX(), obs.GetHaltReason())
	}
}

func TestStopsWhenHeartbeatsStop(t *testing.T) {
	r, c := newTestRobot()
	drive(r, c, 1, 0, time.Second, true)
	drive(r, c, 1, 0, r.cfg.HeartbeatTimeout+dt, false)
	obs := r.Observe()
	if obs.GetVelocity().GetLinear().GetX() != 0 || obs.GetHaltReason() != tourianv1.HaltReason_HALT_REASON_DEAD_MAN {
		t.Errorf("without heartbeats: linear = %v, halt = %v", obs.GetVelocity().GetLinear().GetX(), obs.GetHaltReason())
	}

	drive(r, c, 1, 0, 2*dt, true)
	if r.Observe().GetHaltReason() != tourianv1.HaltReason_HALT_REASON_UNSPECIFIED {
		t.Errorf("heartbeats resumed but halt = %v", r.Observe().GetHaltReason())
	}
}

func TestHeldStillUntilFirstHeartbeat(t *testing.T) {
	r, c := newTestRobot()
	drive(r, c, 1, 0, time.Second, false)
	obs := r.Observe()
	if obs.GetPose().GetPosition().GetX() != 0 || obs.GetHaltReason() != tourianv1.HaltReason_HALT_REASON_DEAD_MAN {
		t.Errorf("moved without an operator: x = %v, halt = %v", obs.GetPose().GetPosition().GetX(), obs.GetHaltReason())
	}
}

func TestEmergencyStopPreemptsTheQueue(t *testing.T) {
	r, c := newTestRobot()
	drive(r, c, 1, 0, time.Second, true)
	for seq := uint64(1); seq <= 3; seq++ {
		cmd := command(1, 0)
		cmd.Sequence = seq
		r.Command(cmd)
	}
	r.Stop(&tourianv1.Stop{})
	c.t = c.t.Add(dt)
	r.Step(dt)

	obs := r.Observe()
	if obs.GetVelocity().GetLinear().GetX() != 0 || obs.GetHaltReason() != tourianv1.HaltReason_HALT_REASON_EMERGENCY_STOP {
		t.Errorf("after stop: linear = %v, halt = %v", obs.GetVelocity().GetLinear().GetX(), obs.GetHaltReason())
	}
	if obs.GetLastCommandSequence() != 0 {
		t.Errorf("a queued command was applied after the stop: sequence %d", obs.GetLastCommandSequence())
	}

	drive(r, c, 1, 0, 2*dt, true)
	if r.Observe().GetHaltReason() != tourianv1.HaltReason_HALT_REASON_UNSPECIFIED {
		t.Errorf("a new command should clear the stop, halt = %v", r.Observe().GetHaltReason())
	}
}

func TestAcknowledgesWithTheCommandStamp(t *testing.T) {
	r, c := newTestRobot()
	cmd := command(0, 0)
	cmd.Sequence = 7
	cmd.Header = (&Robot{now: c.now}).header()
	c.t = c.t.Add(3 * time.Millisecond)
	ack := r.Command(cmd)
	if ack.GetSequence() != 7 || !ack.GetCommandStamp().AsTime().Equal(cmd.GetHeader().GetStamp().AsTime()) {
		t.Errorf("ack = %v", ack)
	}
	if ack.GetHeader().GetStamp().AsTime().Sub(cmd.GetHeader().GetStamp().AsTime()) != 3*time.Millisecond {
		t.Errorf("ack stamp should be the arrival time")
	}
}

func TestBatteryDrains(t *testing.T) {
	r, c := newTestRobot()
	drive(r, c, 1, 0, 36*time.Second, true)
	near(t, "battery after 1% of an hour at full speed", r.Observe().GetBattery().GetPercentage(), 1-(r.cfg.IdleDrainPerHour+r.cfg.DriveDrainPerHour)/100)
}

func TestStartsAtItsStartPose(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StartX, cfg.StartY, cfg.StartTheta = 1, -2, math.Pi/2
	r := New(cfg, time.Now)
	obs := r.Observe()
	near(t, "x", obs.GetPose().GetPosition().GetX(), 1)
	near(t, "y", obs.GetPose().GetPosition().GetY(), -2)
	near(t, "heading", 2*math.Atan2(obs.GetPose().GetOrientation().GetZ(), obs.GetPose().GetOrientation().GetW()), math.Pi/2)
}

func TestCapabilities(t *testing.T) {
	r, _ := newTestRobot()
	caps := r.Capabilities()
	if caps.GetRobotId() != "zebes-1" || len(caps.GetJointNames()) != 2 || caps.GetVelocityLimits().GetMaxLinear() != 1 {
		t.Errorf("capabilities = %v", caps)
	}
	if caps.GetCommandTimeout().AsDuration() != 500*time.Millisecond || caps.GetHeartbeatTimeout().AsDuration() != time.Second {
		t.Errorf("timeouts = %v / %v", caps.GetCommandTimeout().AsDuration(), caps.GetHeartbeatTimeout().AsDuration())
	}
}
