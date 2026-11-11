package episode

import (
	"context"
	"log"
	"regexp"
	"sync/atomic"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/hub"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Platform is what the recorder needs from the hub: a word to every console, and a robot's pictures.
type Platform interface {
	Publish(message proto.Message)
	RequestVideo(robotID, peerID string, enabled bool)
}

// Configs tells the recorder which configurations are current, so an episode says what it was recorded on and for.
type Configs interface {
	LatestRobot(robotID string) *tourianv1.RobotConfig
	LatestTask(taskID string) *tourianv1.TaskConfig
}

// A Sink is one layout an episode is written in. Every episode goes to every sink the recorder has.
type Sink interface {
	Begin(episode *tourianv1.Episode) error
	Frame(episode *tourianv1.Episode, frame *tourianv1.EpisodeFrame) error
	Picture(episode *tourianv1.Episode, frame *tourianv1.VideoFrame, offset time.Duration) error
	End(episode *tourianv1.Episode) error
	Flag(episodeID string, bad bool, note string) error
}

const (
	peerID         = "recorder"
	inboxDepth     = 4096
	statusInterval = time.Second
)

// A Recorder listens to everything the hub routes and, for each robot it has been told to record, writes an episode: a frame per observation, carrying the command in force and the latest picture.
type Recorder struct {
	sinks    []Sink
	platform Platform
	configs  Configs
	now      func() time.Time
	inbox    chan *tourianv1.Envelope
	dropped  atomic.Int64
	active   map[string]*recording
	holders  map[string]*tourianv1.Controller
}

type recording struct {
	episode     *tourianv1.Episode
	sinks       []Sink
	startedAt   time.Time
	lastCommand *tourianv1.VelocityCommand
	lastJoints  *tourianv1.JointCommand
	lastVideo   uint64
}

func NewRecorder(platform Platform, now func() time.Time, sinks ...Sink) *Recorder {
	return &Recorder{
		sinks:    sinks,
		platform: platform,
		now:      now,
		inbox:    make(chan *tourianv1.Envelope, inboxDepth),
		active:   map[string]*recording{},
		holders:  map[string]*tourianv1.Controller{},
	}
}

// WithConfigs makes the recorder stamp the current configuration versions on each episode.
func (r *Recorder) WithConfigs(configs Configs) *Recorder {
	r.configs = configs
	return r
}

// Tap hands a message to the recorder without waiting; one that finds the inbox full is dropped and counted.
func (r *Recorder) Tap(_ hub.Source, env *tourianv1.Envelope) {
	select {
	case r.inbox <- env:
	default:
		r.dropped.Add(1)
	}
}

// Run handles the inbox and reports status until ctx ends, then closes any episode still open.
func (r *Recorder) Run(ctx context.Context) {
	ticker := time.NewTicker(statusInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			for robotID := range r.active {
				r.stop(robotID)
			}
			return
		case env := <-r.inbox:
			r.Handle(env)
		case <-ticker.C:
			for _, rec := range r.active {
				r.platform.Publish(r.status(rec, true))
			}
		}
	}
}

// Handle applies one message; Run calls it from one goroutine, tests call it directly.
func (r *Recorder) Handle(env *tourianv1.Envelope) {
	rec := r.active[ws.RobotID(env)]
	switch m := env.GetMessage().(type) {
	case *tourianv1.Envelope_RecordRequest:
		if m.RecordRequest.GetStart() {
			r.start(m.RecordRequest)
		} else {
			r.stop(m.RecordRequest.GetRobotId())
		}
	case *tourianv1.Envelope_EpisodeFlag:
		r.flag(m.EpisodeFlag)
	case *tourianv1.Envelope_VelocityCommand:
		if rec != nil {
			rec.lastCommand = m.VelocityCommand
		}
	case *tourianv1.Envelope_JointCommand:
		if rec != nil {
			rec.lastJoints = m.JointCommand
		}
	case *tourianv1.Envelope_Observation:
		if rec != nil {
			r.observe(rec, m.Observation)
		}
	case *tourianv1.Envelope_VideoFrame:
		if rec != nil {
			r.picture(rec, m.VideoFrame)
		}
	case *tourianv1.Envelope_ControlState:
		r.holders[m.ControlState.GetRobotId()] = m.ControlState.GetHolder()
	}
}

func (r *Recorder) start(request *tourianv1.RecordRequest) {
	robotID := request.GetRobotId()
	if _, recording := r.active[robotID]; recording || robotID == "" {
		return
	}
	now := r.now()
	id := now.UTC().Format("20060102-150405.000") + "-" + SafeName(robotID)
	rec := &recording{
		episode: &tourianv1.Episode{
			Id:        id,
			RobotId:   robotID,
			Task:      request.GetTask(),
			TaskId:    request.GetTaskId(),
			StartedAt: timestamppb.New(now),
		},
		startedAt: now,
	}
	if r.configs != nil {
		rec.episode.RobotConfigVersion = r.configs.LatestRobot(robotID).GetVersion()
		if task := r.configs.LatestTask(request.GetTaskId()); task != nil {
			rec.episode.TaskConfigVersion = task.GetVersion()
			if rec.episode.Task == "" {
				rec.episode.Task = task.GetName()
			}
		}
	}
	for _, sink := range r.sinks {
		if err := sink.Begin(rec.episode); err != nil {
			log.Printf("episode: %s goes without %T: %v", id, sink, err)
			continue
		}
		rec.sinks = append(rec.sinks, sink)
	}
	if len(rec.sinks) == 0 {
		log.Printf("episode: cannot start %s: no sink could begin", id)
		return
	}
	r.active[robotID] = rec
	r.platform.RequestVideo(robotID, peerID, true)
	r.platform.Publish(r.status(rec, true))
	log.Printf("episode: recording %s", id)
}

func (r *Recorder) stop(robotID string) {
	rec := r.active[robotID]
	if rec == nil {
		return
	}
	delete(r.active, robotID)
	r.platform.RequestVideo(robotID, peerID, false)
	rec.episode.EndedAt = timestamppb.New(r.now())
	for _, sink := range rec.sinks {
		if err := sink.End(rec.episode); err != nil {
			log.Printf("episode: closing %s: %v", rec.episode.GetId(), err)
		}
	}
	r.platform.Publish(r.status(rec, false))
	log.Printf("episode: %s ended with %d frames and %d pictures", rec.episode.GetId(), rec.episode.GetFrames(), rec.episode.GetVideoFrames())
}

func (r *Recorder) observe(rec *recording, observation *tourianv1.Observation) {
	frame := &tourianv1.EpisodeFrame{
		Index:         rec.episode.GetFrames(),
		Offset:        durationpb.New(r.offset(rec, observation.GetHeader().GetStamp())),
		Observation:   observation,
		Action:        rec.lastCommand,
		JointAction:   rec.lastJoints,
		VideoSequence: rec.lastVideo,
		Controller:    r.holders[observation.GetRobotId()],
	}
	for _, sink := range rec.sinks {
		if err := sink.Frame(rec.episode, frame); err != nil {
			log.Printf("episode: writing frame: %v", err)
		}
	}
	rec.episode.Frames++
}

func (r *Recorder) picture(rec *recording, frame *tourianv1.VideoFrame) {
	for _, sink := range rec.sinks {
		if err := sink.Picture(rec.episode, frame, r.offset(rec, frame.GetHeader().GetStamp())); err != nil {
			log.Printf("episode: writing picture: %v", err)
		}
	}
	rec.lastVideo = frame.GetSequence()
	rec.episode.VideoFrames++
}

// The offset of a frame is taken from its own stamp when it has one, so a late delivery lands where it happened.
func (r *Recorder) offset(rec *recording, stamp *timestamppb.Timestamp) time.Duration {
	at := r.now()
	if stamp != nil {
		at = stamp.AsTime()
	}
	return max(0, at.Sub(rec.startedAt))
}

func (r *Recorder) flag(flag *tourianv1.EpisodeFlag) {
	for _, rec := range r.active {
		if rec.episode.GetId() == flag.GetEpisodeId() {
			rec.episode.Bad = flag.GetBad()
			rec.episode.Note = flag.GetNote()
			return
		}
	}
	for _, sink := range r.sinks {
		if err := sink.Flag(flag.GetEpisodeId(), flag.GetBad(), flag.GetNote()); err != nil {
			log.Printf("episode: flagging %s: %v", flag.GetEpisodeId(), err)
		}
	}
}

func (r *Recorder) status(rec *recording, recording bool) *tourianv1.EpisodeStatus {
	return &tourianv1.EpisodeStatus{
		RobotId:     rec.episode.GetRobotId(),
		EpisodeId:   rec.episode.GetId(),
		Recording:   recording,
		StartedAt:   rec.episode.GetStartedAt(),
		Frames:      rec.episode.GetFrames(),
		VideoFrames: rec.episode.GetVideoFrames(),
	}
}

var unsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// SafeName keeps an id usable as a file name.
func SafeName(s string) string {
	return unsafe.ReplaceAllString(s, "_")
}
