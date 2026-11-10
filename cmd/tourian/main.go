// The platform: robots connect on one endpoint, consoles on another, and the hub routes between them.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/vlumi/tourian/emmi/zebes"
	"github.com/vlumi/tourian/hub"
)

func main() {
	addr := flag.String("addr", ":8080", "address the platform listens on")
	sim := flag.Bool("sim", false, "also run the simulated robot in this process, connected to this platform")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:              *addr,
		Handler:           newHandler(hub.New()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if *sim {
		go func() {
			if err := zebes.Run(ctx, robotURL(*addr), zebes.DefaultConfig()); err != nil && ctx.Err() == nil {
				log.Printf("zebes: %v", err)
			}
		}()
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
