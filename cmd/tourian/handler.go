package main

import (
	"net/http"

	"github.com/vlumi/tourian/hub"
)

func newHandler(h *hub.Hub) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /robot", h.ServeRobot)
	mux.HandleFunc("GET /camera", h.ServeCamera)
	mux.HandleFunc("GET /console", h.ServeConsole)
	return mux
}
