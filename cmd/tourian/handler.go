package main

import (
	"net/http"

	"github.com/vlumi/tourian/config"
	"github.com/vlumi/tourian/episode"
	"github.com/vlumi/tourian/hub"
)

func newHandler(h *hub.Hub, episodes string, configs *config.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /robot", h.ServeRobot)
	mux.HandleFunc("GET /camera", h.ServeCamera)
	mux.HandleFunc("GET /console", h.ServeConsole)
	mux.Handle("GET /episodes", episode.Handler(episodes))
	mux.Handle("GET /episodes/", episode.Handler(episodes))
	mux.Handle("/config/", config.Handler(configs, h))
	return allowAnyOrigin(mux)
}

// The console is served from another port than the platform, so its fetches are cross-origin; nothing here is private until the platform leaves the laptop.
func allowAnyOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
