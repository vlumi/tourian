// Package script is a controller that drives a pattern, with no eyes: a square, by the clock on the observations.
package script

import (
	"context"
	"math"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
)

const (
	legDuration  = 2 * time.Second
	turnDuration = 2 * time.Second
	speed        = 0.5
	turnRate     = math.Pi / 2 / 2
)

// Square drives a leg, turns a quarter, and repeats; the phase comes from the observation's stamp, so the pattern needs no state.
type Square struct{}

func (Square) Describe() *tourianv1.Controller {
	return &tourianv1.Controller{Id: "square", Kind: tourianv1.ControllerKind_CONTROLLER_KIND_SCRIPT, Name: "Square"}
}

func (Square) Act(_ context.Context, observation *tourianv1.Observation) (*tourianv1.Twist, error) {
	at := observation.GetHeader().GetStamp().AsTime()
	return TwistAt(at), nil
}

func TwistAt(at time.Time) *tourianv1.Twist {
	phase := time.Duration(at.UnixNano()) % (legDuration + turnDuration)
	if phase < legDuration {
		return &tourianv1.Twist{Linear: &tourianv1.Vector3{X: speed}, Angular: &tourianv1.Vector3{}}
	}
	return &tourianv1.Twist{Linear: &tourianv1.Vector3{}, Angular: &tourianv1.Vector3{Z: turnRate}}
}
