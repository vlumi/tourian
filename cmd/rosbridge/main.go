// The rosbridge adapter as its own process: a ROS 2 robot on one side, the platform on the other.
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"strings"
	"syscall"

	"github.com/vlumi/tourian/emmi/rosbridge"
)

func main() {
	platform := flag.String("platform", "ws://localhost:8080/api/robot", "the platform's robot endpoint")
	cfg := rosbridge.DefaultConfig()
	flag.StringVar(&cfg.RosbridgeURL, "rosbridge", cfg.RosbridgeURL, "the robot's rosbridge websocket")
	flag.StringVar(&cfg.RobotID, "id", cfg.RobotID, "robot id")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "robot name")
	flag.StringVar(&cfg.CmdVelTopic, "cmd-vel", cfg.CmdVelTopic, "topic the velocity commands are published on")
	flag.StringVar(&cfg.OdomTopic, "odom", cfg.OdomTopic, "odometry topic")
	flag.StringVar(&cfg.JointStatesTopic, "joint-states", cfg.JointStatesTopic, "joint states topic")
	flag.StringVar(&cfg.BatteryTopic, "battery", cfg.BatteryTopic, "battery state topic")
	joints := flag.String("joints", "", "comma-separated joint names to announce")
	flag.Float64Var(&cfg.MaxLinear, "max-linear", cfg.MaxLinear, "fastest the robot may be driven, m/s")
	flag.Float64Var(&cfg.MaxAngular, "max-angular", cfg.MaxAngular, "fastest the robot may be turned, rad/s")
	flag.Parse()
	if *joints != "" {
		cfg.JointNames = strings.Split(*joints, ",")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := rosbridge.Run(ctx, *platform, cfg); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
