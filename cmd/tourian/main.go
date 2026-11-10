package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", ":8080", "address the platform listens on")
	flag.Parse()

	server := &http.Server{
		Addr:              *addr,
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("tourian listening on %s", *addr)
	log.Fatal(server.ListenAndServe())
}
