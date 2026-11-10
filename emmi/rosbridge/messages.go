package rosbridge

import (
	"encoding/json"
	"time"
)

// The rosbridge protocol: one JSON object per frame, with an op and, for topics, a message in ROS 2's JSON shape.
type operation struct {
	Op    string          `json:"op"`
	Topic string          `json:"topic,omitempty"`
	Type  string          `json:"type,omitempty"`
	Msg   json.RawMessage `json:"msg,omitempty"`
}

type rosTime struct {
	Sec     int64 `json:"sec"`
	Nanosec int64 `json:"nanosec"`
}

func (t rosTime) Time() time.Time {
	if t.Sec == 0 && t.Nanosec == 0 {
		return time.Time{}
	}
	return time.Unix(t.Sec, t.Nanosec)
}

type rosHeader struct {
	Stamp   rosTime `json:"stamp"`
	FrameID string  `json:"frame_id"`
}

type vector3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type quaternion struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
	W float64 `json:"w"`
}

type twist struct {
	Linear  vector3 `json:"linear"`
	Angular vector3 `json:"angular"`
}

type odometry struct {
	Header       rosHeader `json:"header"`
	ChildFrameID string    `json:"child_frame_id"`
	Pose         struct {
		Pose struct {
			Position    vector3    `json:"position"`
			Orientation quaternion `json:"orientation"`
		} `json:"pose"`
	} `json:"pose"`
	Twist struct {
		Twist twist `json:"twist"`
	} `json:"twist"`
}

type jointState struct {
	Header   rosHeader `json:"header"`
	Name     []string  `json:"name"`
	Position []float64 `json:"position"`
	Velocity []float64 `json:"velocity"`
	Effort   []float64 `json:"effort"`
}

type batteryState struct {
	Header     rosHeader `json:"header"`
	Voltage    float64   `json:"voltage"`
	Percentage float64   `json:"percentage"`
}

const (
	twistType      = "geometry_msgs/msg/Twist"
	odometryType   = "nav_msgs/msg/Odometry"
	jointStateType = "sensor_msgs/msg/JointState"
	batteryType    = "sensor_msgs/msg/BatteryState"
)
