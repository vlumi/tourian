// The simulated robot as its own process, connecting to a platform.
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/vlumi/tourian/emmi/zebes"
)

func main() {
	platform := flag.String("platform", "ws://localhost:8080/robot", "the platform's robot endpoint")
	cfg := zebes.DefaultConfig()
	flag.StringVar(&cfg.RobotID, "id", cfg.RobotID, "robot id")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "robot name")
	flag.Float64Var(&cfg.StartX, "x", 0, "start position x in meters")
	flag.Float64Var(&cfg.StartY, "y", 0, "start position y in meters")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := zebes.Run(ctx, *platform, cfg); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
