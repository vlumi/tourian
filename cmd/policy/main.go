// A policy server to stand in for a learned one: it steers the robot home to the origin. It speaks the protocol the policy controller speaks, an observation in and a twist out as JSON, so a real policy server replaces it without the platform noticing.
package main

import (
	"flag"
	"io"
	"log"
	"math"
	"net/http"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	speed       = 0.5
	turnGain    = 1.5
	maxTurn     = math.Pi / 2
	homeRadius  = 0.2
	alignedWith = 0.5
)

func main() {
	addr := flag.String("addr", ":8091", "address the policy server listens on")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /act", act)
	log.Printf("policy listening on %s", *addr)
	log.Fatal((&http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}).ListenAndServe())
}

func act(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	observation := &tourianv1.Observation{}
	if err := protojson.Unmarshal(body, observation); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data, _ := protojson.Marshal(Home(observation))
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// Home turns toward the origin and drives when roughly facing it, stopping once there.
func Home(observation *tourianv1.Observation) *tourianv1.Twist {
	position := observation.GetPose().GetPosition()
	orientation := observation.GetPose().GetOrientation()
	yaw := 2 * math.Atan2(orientation.GetZ(), orientation.GetW())
	distance := math.Hypot(position.GetX(), position.GetY())
	if distance < homeRadius {
		return &tourianv1.Twist{Linear: &tourianv1.Vector3{}, Angular: &tourianv1.Vector3{}}
	}
	bearing := math.Atan2(-position.GetY(), -position.GetX())
	err := math.Atan2(math.Sin(bearing-yaw), math.Cos(bearing-yaw))
	twist := &tourianv1.Twist{
		Linear:  &tourianv1.Vector3{},
		Angular: &tourianv1.Vector3{Z: math.Max(-maxTurn, math.Min(maxTurn, turnGain*err))},
	}
	if math.Abs(err) < alignedWith {
		twist.Linear.X = speed
	}
	return twist
}
