// The platform: robots connect on one endpoint, consoles on another, and the hub routes between them.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/vlumi/tourian/config"
	"github.com/vlumi/tourian/emmi/zebes"
	"github.com/vlumi/tourian/episode"
	"github.com/vlumi/tourian/episode/lerobot"
	"github.com/vlumi/tourian/hub"
	"github.com/vlumi/tourian/motherbrain"
	"github.com/vlumi/tourian/motherbrain/policy"
	"github.com/vlumi/tourian/motherbrain/script"
)

// The simulator observes at this rate, which becomes the dataset's frame rate.
const observationsPerSecond = 20

func main() {
	addr := flag.String("addr", ":8080", "address the platform listens on")
	sim := flag.Bool("sim", false, "also run the simulated robot in this process, connected to this platform")
	sims := flag.Int("sims", 1, "how many simulated robots -sim runs, each with its own id and start pose")
	data := flag.String("data", "data", "directory the episodes are recorded under")
	dataset := flag.String("lerobot", "tourian", "name of the LeRobot dataset written under the data directory, empty for none")
	policyURL := flag.String("policy", "", "URL of a policy server to offer as a controller, such as http://localhost:8091")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	h := hub.New()
	episodes := filepath.Join(*data, "episodes")
	sinks := []episode.Sink{episode.NewJSONL(episodes)}
	if *dataset != "" {
		sinks = append(sinks, lerobot.New(filepath.Join(*data, "lerobot", *dataset), observationsPerSecond))
	}
	configs := config.NewStore(filepath.Join(*data, "config"), time.Now)
	recorder := episode.NewRecorder(h, time.Now, sinks...).WithConfigs(configs)
	h.AddTap(recorder.Tap)
	go recorder.Run(ctx)

	controllers := []motherbrain.Controller{script.Square{}}
	if *policyURL != "" {
		controllers = append(controllers, policy.New(*policyURL))
	}
	arbiter := motherbrain.NewArbiter(h, time.Now, controllers...)
	h.AddTap(arbiter.Tap)
	go arbiter.Run(ctx)

	server := &http.Server{
		Addr:              *addr,
		Handler:           newHandler(h, episodes, configs),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if *sim {
		for i := range *sims {
			cfg := zebes.DefaultConfig()
			if *sims > 1 {
				cfg.RobotID = fmt.Sprintf("zebes-%d", i+1)
				cfg.Name = fmt.Sprintf("Zebes rover %d", i+1)
				cfg.StartY = float64(*sims-1) - 2*float64(i)
			}
			go func() {
				if err := zebes.Run(ctx, robotURL(*addr), cfg); err != nil && ctx.Err() == nil {
					log.Printf("zebes: %v", err)
				}
			}()
		}
	}

	log.Printf("tourian listening on %s", *addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func robotURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "ws://" + addr + "/robot"
	}
	if host == "" {
		host = "localhost"
	}
	return "ws://" + net.JoinHostPort(host, port) + "/robot"
}
