package main

import (
	"math"
	"testing"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
)

func observation(x, y, yaw float64) *tourianv1.Observation {
	return &tourianv1.Observation{Pose: &tourianv1.Pose{
		Position:    &tourianv1.Point{X: x, Y: y},
		Orientation: &tourianv1.Quaternion{Z: math.Sin(yaw / 2), W: math.Cos(yaw / 2)},
	}}
}

func TestHome(t *testing.T) {
	if twist := Home(observation(0.1, 0, 0)); twist.GetLinear().GetX() != 0 || twist.GetAngular().GetZ() != 0 {
		t.Errorf("at home: %v", twist)
	}
	if twist := Home(observation(2, 0, math.Pi)); twist.GetLinear().GetX() != speed || math.Abs(twist.GetAngular().GetZ()) > 1e-9 {
		t.Errorf("facing home from the right: %v", twist)
	}
	if twist := Home(observation(2, 0, 0)); twist.GetLinear().GetX() != 0 || math.Abs(twist.GetAngular().GetZ()) != maxTurn {
		t.Errorf("facing away: %v", twist)
	}
	if twist := Home(observation(0, 2, 0)); twist.GetAngular().GetZ() >= 0 {
		t.Errorf("home is clockwise from here: %v", twist)
	}
}
