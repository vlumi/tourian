// A simulated ROS 2 robot behind the rosbridge protocol, as its own process, for driving the rosbridge adapter without ROS 2 at hand.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/vlumi/tourian/emmi/rosbridge/rosbridgetest"
)

func main() {
	addr := flag.String("addr", ":9090", "address the rosbridge listens on")
	flag.Parse()
	log.Printf("rossim: rosbridge listening on %s", *addr)
	log.Fatal((&http.Server{Addr: *addr, Handler: rosbridgetest.New(), ReadHeaderTimeout: 5 * time.Second}).ListenAndServe())
}
