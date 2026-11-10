// Package hub is the platform's switchboard: robot adapters announce themselves on one endpoint, consoles join on another, and every message is routed by robot id without the hub knowing what is on either side.
package hub

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/proto"
)

const (
	outboundDepth = 256
	writeTimeout  = time.Second
)

type Hub struct {
	mu           sync.Mutex
	robots       map[string]*peer
	consoles     map[*peer]struct{}
	capabilities map[string]*tourianv1.Capabilities
}

func New() *Hub {
	return &Hub{
		robots:       map[string]*peer{},
		consoles:     map[*peer]struct{}{},
		capabilities: map[string]*tourianv1.Capabilities{},
	}
}

// ServeRobot accepts a robot adapter, which must announce its capabilities in its first frame.
func (h *Hub) ServeRobot(w http.ResponseWriter, r *http.Request) {
	p, err := accept(w, r)
	if err != nil {
		return
	}
	defer p.close()

	env, err := ws.Receive(r.Context(), p.conn)
	if err != nil || env.GetCapabilities() == nil {
		log.Printf("hub: robot from %s did not announce itself", r.RemoteAddr)
		return
	}
	id := env.GetCapabilities().GetRobotId()
	h.announce(id, p, env)
	defer h.leaveRobot(id, p)
	log.Printf("hub: robot %q connected from %s", id, r.RemoteAddr)

	for {
		env, err := ws.Receive(r.Context(), p.conn)
		if err != nil {
			log.Printf("hub: robot %q disconnected: %v", id, err)
			return
		}
		switch env.GetMessage().(type) {
		case *tourianv1.Envelope_Observation, *tourianv1.Envelope_CommandAck:
			h.broadcast(env)
		case *tourianv1.Envelope_Capabilities:
			h.announce(id, p, env)
		}
	}
}

// ServeConsole accepts a console, tells it about every robot known, and routes what it sends to the robot it names.
func (h *Hub) ServeConsole(w http.ResponseWriter, r *http.Request) {
	p, err := accept(w, r)
	if err != nil {
		return
	}
	defer p.close()
	h.join(p)
	defer h.leaveConsole(p)
	log.Printf("hub: console connected from %s", r.RemoteAddr)

	for {
		env, err := ws.Receive(r.Context(), p.conn)
		if err != nil {
			log.Printf("hub: console from %s disconnected: %v", r.RemoteAddr, err)
			return
		}
		switch env.GetMessage().(type) {
		case *tourianv1.Envelope_VelocityCommand, *tourianv1.Envelope_Heartbeat, *tourianv1.Envelope_Stop:
			if robot := h.robot(ws.RobotID(env)); robot != nil {
				robot.send(env)
			}
		}
	}
}

func (h *Hub) announce(id string, p *peer, env *tourianv1.Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if previous, ok := h.robots[id]; ok && previous != p {
		previous.close()
	}
	h.robots[id] = p
	h.capabilities[id] = env.GetCapabilities()
	for console := range h.consoles {
		console.send(env)
	}
}

func (h *Hub) leaveRobot(id string, p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.robots[id] == p {
		delete(h.robots, id)
		delete(h.capabilities, id)
	}
}

func (h *Hub) join(p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.consoles[p] = struct{}{}
	for _, caps := range h.capabilities {
		env, _ := ws.Wrap(caps)
		p.send(env)
	}
}

func (h *Hub) leaveConsole(p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.consoles, p)
}

func (h *Hub) robot(id string) *peer {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.robots[id]
}

func (h *Hub) broadcast(env *tourianv1.Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for console := range h.consoles {
		console.send(env)
	}
}

// A peer writes from its own goroutine so that routing never blocks on a slow connection; one that cannot keep up is dropped, and a robot behind it stops on its own timeouts.
type peer struct {
	conn   *websocket.Conn
	out    chan proto.Message
	closed chan struct{}
	once   sync.Once
}

func accept(w http.ResponseWriter, r *http.Request) (*peer, error) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return nil, err
	}
	p := &peer{conn: conn, out: make(chan proto.Message, outboundDepth), closed: make(chan struct{})}
	go p.writeLoop()
	return p, nil
}

func (p *peer) writeLoop() {
	for {
		select {
		case <-p.closed:
			return
		case message := <-p.out:
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := ws.Send(ctx, p.conn, message)
			cancel()
			if err != nil {
				p.close()
				return
			}
		}
	}
}

func (p *peer) send(message proto.Message) {
	select {
	case p.out <- message:
	default:
		p.close()
	}
}

func (p *peer) close() {
	p.once.Do(func() {
		close(p.closed)
		_ = p.conn.CloseNow()
	})
}
