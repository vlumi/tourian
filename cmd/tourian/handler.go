package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"

	"github.com/vlumi/tourian/config"
	"github.com/vlumi/tourian/episode"
	"github.com/vlumi/tourian/hub"
)

// newHandler mounts the platform's endpoints under /api and, when the console is served behind it, hands every other path to the console, so a phone needs one address and one certificate.
func newHandler(h *hub.Hub, episodes string, configs *config.Store, console *url.URL, certs string) http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	api.HandleFunc("GET /robot", h.ServeRobot)
	api.HandleFunc("GET /camera", h.ServeCamera)
	api.HandleFunc("GET /console", h.ServeConsole)
	browse := episode.Handler(episodes)
	api.Handle("GET /episodes", browse)
	api.Handle("GET /episodes/", browse)
	api.Handle("/config/", config.Handler(configs, h))
	if certs != "" {
		api.HandleFunc("GET /ca.pem", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-x509-ca-cert")
			http.ServeFile(w, r, filepath.Join(certs, authorityFile))
		})
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api))
	if console != nil {
		mux.Handle("/", consoleProxy(console))
	}
	return allowOrigins(h, mux)
}

// consoleProxy passes requests to the console's own server as if they came from this machine, so its development server takes them without a cross-origin complaint.
func consoleProxy(console *url.URL) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(console)
			r.Out.Host = console.Host
			r.Out.Header.Del("Origin")
		},
	}
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
