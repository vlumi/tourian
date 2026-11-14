// The platform: robots connect on one endpoint, consoles on another, and the hub routes between them.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/vlumi/tourian/config"
	"github.com/vlumi/tourian/emmi/zebes"
	"github.com/vlumi/tourian/episode"
	"github.com/vlumi/tourian/episode/lerobot"
	"github.com/vlumi/tourian/episode/mcap"
	"github.com/vlumi/tourian/hub"
	"github.com/vlumi/tourian/motherbrain"
	"github.com/vlumi/tourian/motherbrain/policy"
	"github.com/vlumi/tourian/motherbrain/script"
)

// The simulator observes at this rate, which becomes the dataset's frame rate.
const observationsPerSecond = 20

func main() {
	addr := flag.String("addr", ":8080", "address the platform listens on")
	httpsAddr := flag.String("https", ":8443", "address the platform also listens on with its own certificate, kept under the data directory; empty for none")
	consoleURL := flag.String("console", "http://localhost:1499", "the console's server, served behind the platform so a phone needs one address; empty for none")
	lan := flag.String("lan", "", "the host phones reach the platform by, default the first address on the local network")
	sim := flag.Bool("sim", false, "also run the simulated robot in this process, connected to this platform")
	sims := flag.Int("sims", 1, "how many simulated robots -sim runs, each with its own id and start pose")
	data := flag.String("data", "data", "directory the episodes are recorded under")
	dataset := flag.String("lerobot", "tourian", "name of the LeRobot dataset written under the data directory, empty for none")
	writeMcap := flag.Bool("mcap", true, "also write each episode as an MCAP file for Foxglove under the data directory")
	var policyURLs []string
	flag.Func("policy", "URL of a policy or script server to offer as a controller, such as http://localhost:8091; may repeat", func(u string) error {
		policyURLs = append(policyURLs, u)
		return nil
	})
	origins := flag.String("origins", "", "browser origins to let in besides this host and localhost, as host patterns separated by commas, such as phone.local:*")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	h := hub.New()
	if *origins != "" {
		h.Origins = strings.Split(*origins, ",")
	}
	var console *url.URL
	if *consoleURL != "" {
		var err error
		if console, err = url.Parse(*consoleURL); err != nil {
			log.Fatalf("-console: %v", err)
		}
	}
	certs := ""
	if *httpsAddr != "" {
		certs = filepath.Join(*data, "tls")
	}
	reach := addresses(*lan, *addr, *httpsAddr, console != nil)
	h.LANURL = reach.LAN
	episodes := filepath.Join(*data, "episodes")
	sinks := []episode.Sink{episode.NewJSONL(episodes)}
	if *dataset != "" {
		sinks = append(sinks, lerobot.New(filepath.Join(*data, "lerobot", *dataset), observationsPerSecond))
	}
	if *writeMcap {
		sinks = append(sinks, mcap.New(filepath.Join(*data, "mcap")))
	}
	configs := config.NewStore(filepath.Join(*data, "config"), time.Now)
	recorder := episode.NewRecorder(h, time.Now, sinks...).WithConfigs(configs)
	h.AddTap(recorder.Tap)
	recorded := make(chan struct{})
	go func() {
		defer close(recorded)
		recorder.Run(ctx)
	}()

	controllers := []motherbrain.Controller{script.Square{}}
	for _, u := range policyURLs {
		controllers = append(controllers, policy.New(u))
	}
	arbiter := motherbrain.NewArbiter(h, time.Now, controllers...)
	h.AddTap(arbiter.Tap)
	h.Admit = arbiter.Admit
	go arbiter.Run(ctx)

	handler := newHandler(h, episodes, configs, console, certs, reach)
	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       time.Minute,
	}
	servers := []*http.Server{server}
	if *httpsAddr != "" {
		keep := &keeper{dir: certs, hosts: lanHosts}
		if err := keep.refresh(); err != nil {
			log.Fatalf("tls: %v", err)
		}
		go keep.follow(ctx, 15*time.Second)
		secure := &http.Server{
			Addr:              *httpsAddr,
			Handler:           handler,
			TLSConfig:         &tls.Config{GetCertificate: keep.get, MinVersion: tls.VersionTLS12},
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       time.Minute,
		}
		servers = append(servers, secure)
		go func() {
			if err := secure.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatal(err)
			}
		}()
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		for _, s := range servers {
			_ = s.Shutdown(shutdownCtx)
		}
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

	log.Printf("tourian listening on %s; the console at http://localhost:%s, phones at %s", *addr, port(*addr, "8080"), h.LANURL)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	// An episode under way is closed on the way out, which can take a while with a video to encode.
	select {
	case <-recorded:
	case <-time.After(30 * time.Second):
		log.Printf("tourian: giving up on closing the episodes")
	}
}

// Where a phone reaches the platform on the local network: over http, where a first visit needs no trust, and over https where it serves one. The console is served behind the platform, else the http address is the console's own.
type reach struct {
	LAN    string `json:"lan"`
	Secure string `json:"secure,omitempty"`
}

func addresses(host, addr, httpsAddr string, console bool) reach {
	if host == "" {
		if ips := lanAddresses(); len(ips) > 0 {
			host = ips[0].String()
		} else {
			host = "localhost"
		}
	}
	r := reach{LAN: "http://" + net.JoinHostPort(host, "1499")}
	if console {
		r.LAN = "http://" + net.JoinHostPort(host, port(addr, "8080"))
	}
	if httpsAddr != "" {
		r.Secure = "https://" + net.JoinHostPort(host, port(httpsAddr, "8443"))
	}
	return r
}

func port(addr, fallback string) string {
	if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
		return p
	}
	return fallback
}

func robotURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "ws://" + addr + "/api/robot"
	}
	if host == "" {
		host = "localhost"
	}
	return "ws://" + net.JoinHostPort(host, port) + "/api/robot"
}
