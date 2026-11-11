// Package rosbridgetest is a simulated ROS 2 robot behind the rosbridge protocol: a differential drive that applies whatever arrives on /cmd_vel until told otherwise, as a plain robot would, and publishes its odometry, joints and battery to whoever subscribes.
package rosbridgetest

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	tick        = 20 * time.Millisecond
	publishRate = 50 * time.Millisecond
)

type Server struct {
	server *httptest.Server
	mu     sync.Mutex
	cmd    twist
	x, y   float64
	theta  float64
	wheels [2]float64
	topics map[string]string
}

type vector3 struct{ X, Y, Z float64 }
type twist struct {
	Linear  vector3 `json:"linear"`
	Angular vector3 `json:"angular"`
}

// New starts the robot on a port of its own, for tests.
func New() *Server {
	s := NewRobot()
	s.server = httptest.NewServer(s)
	return s
}

// NewRobot is the robot without a listener; it serves HTTP wherever it is mounted.
func NewRobot() *Server {
	return &Server{topics: map[string]string{}}
}

func (s *Server) URL() string { return "ws" + strings.TrimPrefix(s.server.URL, "http") }

func (s *Server) Close() {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.serve(w, r) }

// Command is the last twist received on /cmd_vel.
func (s *Server) Command() (linear, angular float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cmd.Linear.X, s.cmd.Angular.Z
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer func() { _ = conn.CloseNow() }()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	subscribed := make(chan string, 8)
	go s.publish(ctx, conn, subscribed)
	go s.integrate(ctx)
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var op struct {
			Op    string          `json:"op"`
			Topic string          `json:"topic"`
			Type  string          `json:"type"`
			Msg   json.RawMessage `json:"msg"`
		}
		if json.Unmarshal(data, &op) != nil {
			continue
		}
		switch op.Op {
		case "subscribe":
			s.mu.Lock()
			s.topics[op.Topic] = op.Type
			s.mu.Unlock()
			subscribed <- op.Topic
		case "publish":
			var t twist
			if json.Unmarshal(op.Msg, &t) == nil {
				s.mu.Lock()
				s.cmd = t
				s.mu.Unlock()
			}
		}
	}
}

func (s *Server) integrate(ctx context.Context) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			dt := tick.Seconds()
			s.theta += s.cmd.Angular.Z * dt
			s.x += s.cmd.Linear.X * math.Cos(s.theta) * dt
			s.y += s.cmd.Linear.X * math.Sin(s.theta) * dt
			s.wheels[0] += (s.cmd.Linear.X - s.cmd.Angular.Z*0.2) / 0.1 * dt
			s.wheels[1] += (s.cmd.Linear.X + s.cmd.Angular.Z*0.2) / 0.1 * dt
			s.mu.Unlock()
		}
	}
}

func (s *Server) publish(ctx context.Context, conn *websocket.Conn, subscribed <-chan string) {
	ticker := time.NewTicker(publishRate)
	defer ticker.Stop()
	active := map[string]bool{}
	for {
		select {
		case <-ctx.Done():
			return
		case topic := <-subscribed:
			active[topic] = true
		case now := <-ticker.C:
			s.mu.Lock()
			stamp := map[string]int64{"sec": now.Unix(), "nanosec": int64(now.Nanosecond())}
			header := map[string]any{"stamp": stamp, "frame_id": "odom"}
			messages := map[string]map[string]any{}
			for topic, kind := range s.topics {
				if !active[topic] {
					continue
				}
				switch kind {
				case "nav_msgs/msg/Odometry":
					messages[topic] = map[string]any{
						"header": header, "child_frame_id": "base_link",
						"pose":  map[string]any{"pose": map[string]any{"position": map[string]float64{"x": s.x, "y": s.y, "z": 0}, "orientation": map[string]float64{"x": 0, "y": 0, "z": math.Sin(s.theta / 2), "w": math.Cos(s.theta / 2)}}},
						"twist": map[string]any{"twist": s.cmd},
					}
				case "sensor_msgs/msg/JointState":
					messages[topic] = map[string]any{"header": header, "name": []string{"wheel_left", "wheel_right"}, "position": []float64{s.wheels[0], s.wheels[1]}, "velocity": []float64{}, "effort": []float64{}}
				case "sensor_msgs/msg/BatteryState":
					messages[topic] = map[string]any{"header": header, "voltage": 12.1, "percentage": 0.8}
				}
			}
			s.mu.Unlock()
			for topic, msg := range messages {
				data, _ := json.Marshal(map[string]any{"op": "publish", "topic": topic, "msg": msg})
				if conn.Write(ctx, websocket.MessageText, data) != nil {
					return
				}
			}
		}
	}
}
