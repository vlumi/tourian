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
	browse := episode.Handler(episodes)
	mux.Handle("GET /episodes", browse)
	mux.Handle("GET /episodes/", browse)
	mux.Handle("/config/", config.Handler(configs, h))
	return allowOrigins(h, mux)
}

// The console is served from another port than the platform, so its fetches are cross-origin; the hub's rule for browsers says which origins are let in.
func allowOrigins(h *hub.Hub, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && h.AllowsOrigin(r, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
