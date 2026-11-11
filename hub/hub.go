// Package hub is the platform's switchboard: robot adapters and cameras announce themselves on their endpoints, consoles join on another, and every message is routed by robot id, and for signaling by console id, without the hub knowing what is on either side.
package hub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/proto"
)

const (
	outboundDepth   = 256
	writeTimeout    = time.Second
	announceTimeout = 5 * time.Second
)

type Hub struct {
	// Origins are host patterns, like "phone.local:*", whose browsers may connect and fetch besides those on the platform's own host or localhost; clients that send no Origin, like the adapters, are not browsers and pass.
	Origins []string
	// Admit says whether a console may drive the robot its command, joint command or heartbeat names; nil admits everyone. What it refuses is dropped before the taps see it.
	Admit        func(consoleID string, env *tourianv1.Envelope) bool
	mu           sync.Mutex
	robots       map[string]*peer
	cameras      map[string]*peer
	consoles     map[string]*peer
	capabilities map[string]*tourianv1.Capabilities
	announces    map[string]*tourianv1.CameraAnnounce
	viewers      map[string]map[string]bool
	retained     map[string]*tourianv1.Envelope
	taps         map[int]Tap
	nextTap      int
}

// Where a message came from: a robot, a camera or a console by its id, or the platform itself.
type Source struct {
	Kind SourceKind
	ID   string
}

type SourceKind int

const (
	SourceRobot SourceKind = iota
	SourceCamera
	SourceConsole
	SourcePlatform
)

// A Tap sees every message the hub receives from any peer, before routing, and everything the platform publishes, on the sender's goroutine; it must not block.
type Tap func(from Source, env *tourianv1.Envelope)

func New() *Hub {
	return &Hub{
		robots:       map[string]*peer{},
		cameras:      map[string]*peer{},
		consoles:     map[string]*peer{},
		capabilities: map[string]*tourianv1.Capabilities{},
		announces:    map[string]*tourianv1.CameraAnnounce{},
		viewers:      map[string]map[string]bool{},
		retained:     map[string]*tourianv1.Envelope{},
		taps:         map[int]Tap{},
	}
}

func (h *Hub) AddTap(tap Tap) (remove func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.nextTap
	h.nextTap++
	h.taps[id] = tap
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.taps, id)
	}
}

func (h *Hub) tap(from Source, env *tourianv1.Envelope) {
	h.mu.Lock()
	taps := make([]Tap, 0, len(h.taps))
	for _, t := range h.taps {
		taps = append(taps, t)
	}
	h.mu.Unlock()
	for _, t := range taps {
		t(from, env)
	}
}

// Publish is how the rest of the platform speaks: the message goes to every tap and every console, and the latest of its kind per robot is kept for consoles that join later.
func (h *Hub) Publish(message proto.Message) {
	env, err := ws.Wrap(message)
	if err != nil {
		return
	}
	h.tap(Source{Kind: SourcePlatform}, env)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.retained[fmt.Sprintf("%T/%s", env.GetMessage(), ws.RobotID(env))] = env
	h.broadcastLocked(env)
}

// SendToRobot delivers a message to a robot on behalf of a part of the platform, like a controller that holds it; taps see it as they see a console's, so what a controller tells a robot is recorded.
func (h *Hub) SendToRobot(robotID string, message proto.Message) {
	env, err := ws.Wrap(message)
	if err != nil {
		return
	}
	h.tap(Source{Kind: SourcePlatform}, env)
	if robot := h.robot(robotID); robot != nil {
		robot.send(env)
	}
}

// RequestVideo asks a robot's camera for frames on behalf of a part of the platform named by peerID.
func (h *Hub) RequestVideo(robotID, peerID string, enabled bool) {
	if camera := h.camera(robotID); camera != nil {
		camera.send(mjpegRequest(robotID, peerID, enabled))
	}
}

func mjpegRequest(robotID, peerID string, enabled bool) *tourianv1.VideoRequest {
	return &tourianv1.VideoRequest{RobotId: robotID, PeerId: peerID, Kind: tourianv1.VideoKind_VIDEO_KIND_MJPEG, Enabled: enabled}
}

// ServeRobot accepts a robot adapter, which must announce its capabilities in its first frame.
func (h *Hub) ServeRobot(w http.ResponseWriter, r *http.Request) {
	p, err := h.accept(w, r)
	if err != nil {
		return
	}
	defer p.close()

	env, err := receiveAnnounce(r.Context(), p.conn)
	if err != nil || env.GetCapabilities() == nil {
		log.Printf("hub: robot from %s did not announce itself", r.RemoteAddr)
		return
	}
	id := env.GetCapabilities().GetRobotId()
	h.tap(Source{Kind: SourceRobot, ID: id}, env)
	h.announceRobot(id, p, env)
	defer h.leaveRobot(id, p)
	log.Printf("hub: robot %q connected from %s", id, r.RemoteAddr)

	for {
		env, err := ws.Receive(r.Context(), p.conn)
		if err != nil {
			log.Printf("hub: robot %q disconnected: %v", id, err)
			return
		}
		h.tap(Source{Kind: SourceRobot, ID: id}, env)
		switch env.GetMessage().(type) {
		case *tourianv1.Envelope_Observation, *tourianv1.Envelope_CommandAck:
			h.broadcast(env)
		case *tourianv1.Envelope_Capabilities:
			h.announceRobot(id, p, env)
		}
	}
}

// ServeCamera accepts a robot's camera, which must announce itself in its first frame. Frames go to the consoles that asked for them; signaling goes to the console it names.
func (h *Hub) ServeCamera(w http.ResponseWriter, r *http.Request) {
	p, err := h.accept(w, r)
	if err != nil {
		return
	}
	defer p.close()

	env, err := receiveAnnounce(r.Context(), p.conn)
	if err != nil || env.GetCameraAnnounce() == nil {
		log.Printf("hub: camera from %s did not announce itself", r.RemoteAddr)
		return
	}
	id := env.GetCameraAnnounce().GetRobotId()
	h.announceCamera(id, p, env)
	defer h.leaveCamera(id, p)
	log.Printf("hub: camera for %q connected from %s", id, r.RemoteAddr)

	for {
		env, err := ws.Receive(r.Context(), p.conn)
		if err != nil {
			log.Printf("hub: camera for %q disconnected: %v", id, err)
			return
		}
		h.tap(Source{Kind: SourceCamera, ID: id}, env)
		switch env.GetMessage().(type) {
		case *tourianv1.Envelope_VideoFrame:
			h.show(id, env)
		case *tourianv1.Envelope_SessionDescription, *tourianv1.Envelope_IceCandidate:
			if console := h.console(ws.PeerID(env)); console != nil {
				console.send(env)
			}
		case *tourianv1.Envelope_CameraAnnounce:
			h.announceCamera(id, p, env)
		}
	}
}

// ServeConsole accepts a console, tells it its id and about every robot known, and routes what it sends to the robot it names; driving is routed only when admitted.
func (h *Hub) ServeConsole(w http.ResponseWriter, r *http.Request) {
	p, err := h.accept(w, r)
	if err != nil {
		return
	}
	defer p.close()
	id := newPeerID()
	h.join(id, p)
	defer h.leaveConsole(id, p)
	log.Printf("hub: console %s connected from %s", id, r.RemoteAddr)

	for {
		env, err := ws.Receive(r.Context(), p.conn)
		if err != nil {
			log.Printf("hub: console %s disconnected: %v", id, err)
			return
		}
		switch env.GetMessage().(type) {
		case *tourianv1.Envelope_VelocityCommand, *tourianv1.Envelope_JointCommand, *tourianv1.Envelope_Heartbeat:
			if h.Admit != nil && !h.Admit(id, env) {
				continue
			}
		}
		h.tap(Source{Kind: SourceConsole, ID: id}, env)
		switch env.GetMessage().(type) {
		case *tourianv1.Envelope_VelocityCommand, *tourianv1.Envelope_JointCommand, *tourianv1.Envelope_Heartbeat, *tourianv1.Envelope_Stop:
			if robot := h.robot(ws.RobotID(env)); robot != nil {
				robot.send(env)
			}
		case *tourianv1.Envelope_VideoRequest, *tourianv1.Envelope_SessionDescription, *tourianv1.Envelope_IceCandidate:
			ws.SetPeerID(env, id)
			if request := env.GetVideoRequest(); request.GetKind() == tourianv1.VideoKind_VIDEO_KIND_MJPEG {
				h.watch(request.GetRobotId(), id, request.GetEnabled())
			}
			if camera := h.camera(ws.RobotID(env)); camera != nil {
				camera.send(env)
			}
		}
	}
}

func (h *Hub) announceRobot(id string, p *peer, env *tourianv1.Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	replace(h.robots, id, p)
	h.capabilities[id] = env.GetCapabilities()
	h.broadcastLocked(env)
}

func (h *Hub) announceCamera(id string, p *peer, env *tourianv1.Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	replace(h.cameras, id, p)
	h.announces[id] = env.GetCameraAnnounce()
	h.broadcastLocked(env)
}

func replace(peers map[string]*peer, id string, p *peer) {
	if previous, ok := peers[id]; ok && previous != p {
		previous.close()
	}
	peers[id] = p
}

func (h *Hub) leaveRobot(id string, p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.robots[id] == p {
		delete(h.robots, id)
		delete(h.capabilities, id)
	}
}

func (h *Hub) leaveCamera(id string, p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cameras[id] == p {
		delete(h.cameras, id)
		delete(h.announces, id)
	}
}

func (h *Hub) join(id string, p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.consoles[id] = p
	p.send(&tourianv1.Welcome{ConsoleId: id})
	for _, caps := range h.capabilities {
		p.send(caps)
	}
	for _, announce := range h.announces {
		p.send(announce)
	}
	for _, env := range h.retained {
		p.send(env)
	}
}

// A console that leaves is unsubscribed from every camera, since it can no longer ask itself.
func (h *Hub) leaveConsole(id string, p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.consoles[id] != p {
		return
	}
	delete(h.consoles, id)
	for _, viewers := range h.viewers {
		delete(viewers, id)
	}
	for robotID, camera := range h.cameras {
		camera.send(mjpegRequest(robotID, id, false))
	}
}

func (h *Hub) watch(robotID, consoleID string, enabled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if enabled {
		if h.viewers[robotID] == nil {
			h.viewers[robotID] = map[string]bool{}
		}
		h.viewers[robotID][consoleID] = true
	} else {
		delete(h.viewers[robotID], consoleID)
	}
}

func (h *Hub) show(robotID string, frame *tourianv1.Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for consoleID := range h.viewers[robotID] {
		if console := h.consoles[consoleID]; console != nil {
			console.send(frame)
		}
	}
}

func (h *Hub) robot(id string) *peer {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.robots[id]
}

func (h *Hub) camera(id string) *peer {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cameras[id]
}

func (h *Hub) console(id string) *peer {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.consoles[id]
}

func (h *Hub) broadcast(env *tourianv1.Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.broadcastLocked(env)
}

func (h *Hub) broadcastLocked(message proto.Message) {
	for _, console := range h.consoles {
		console.send(message)
	}
}

func newPeerID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// A peer writes from its own goroutine so that routing never blocks on a slow connection; one that cannot keep up is dropped, and a robot behind it stops on its own timeouts.
type peer struct {
	conn   *websocket.Conn
	out    chan proto.Message
	closed chan struct{}
	once   sync.Once
}

// AllowsOrigin says whether a browser at origin may reach the platform: from the platform's own host on any port, from localhost, or from a host in Origins. The WebSocket endpoints apply the same rule.
func (h *Hub) AllowsOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	for _, pattern := range h.originPatterns(r) {
		if ok, _ := path.Match(pattern, u.Host); ok {
			return true
		}
	}
	return false
}

func (h *Hub) originPatterns(r *http.Request) []string {
	host := r.Host
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	patterns := make([]string, 0, 8+len(h.Origins))
	for _, name := range []string{strings.ToLower(host), "localhost", "127.0.0.1", "[::1]"} {
		patterns = append(patterns, name, name+":*")
	}
	return append(patterns, h.Origins...)
}

func (h *Hub) accept(w http.ResponseWriter, r *http.Request) (*peer, error) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.originPatterns(r)})
	if err != nil {
		return nil, err
	}
	p := &peer{conn: conn, out: make(chan proto.Message, outboundDepth), closed: make(chan struct{})}
	go p.writeLoop()
	return p, nil
}

// A peer that does not announce itself within the timeout is not kept waiting for.
func receiveAnnounce(ctx context.Context, conn *websocket.Conn) (*tourianv1.Envelope, error) {
	ctx, cancel := context.WithTimeout(ctx, announceTimeout)
	defer cancel()
	return ws.Receive(ctx, conn)
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
