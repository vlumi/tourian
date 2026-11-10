package episode

import (
	"context"
	"log"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Platform is what the recorder needs from the hub: a word to every console, and a robot's pictures.
type Platform interface {
	Broadcast(message proto.Message)
	RequestVideo(robotID, peerID string, enabled bool)
}

const (
	peerID         = "recorder"
	inboxDepth     = 4096
	statusInterval = time.Second
)

// A Recorder listens to everything the hub routes and, for each robot it has been told to record, writes an episode: a frame per observation, carrying the command in force and the latest picture.
type Recorder struct {
	root     string
	platform Platform
	now      func() time.Time
	inbox    chan *tourianv1.Envelope
	dropped  atomic.Int64
	active   map[string]*recording
}

type recording struct {
	episode     *tourianv1.Episode
	writer      *Writer
	startedAt   time.Time
	lastCommand *tourianv1.VelocityCommand
	lastVideo   uint64
}

func NewRecorder(root string, platform Platform, now func() time.Time) *Recorder {
	return &Recorder{
		root:     root,
		platform: platform,
		now:      now,
		inbox:    make(chan *tourianv1.Envelope, inboxDepth),
		active:   map[string]*recording{},
	}
}

// Tap hands a message to the recorder without waiting; one that finds the inbox full is dropped and counted.
func (r *Recorder) Tap(env *tourianv1.Envelope) {
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
				r.platform.Broadcast(r.status(rec, true))
			}
		}
	}
}

// Handle applies one message; Run calls it from one goroutine, tests call it directly.
func (r *Recorder) Handle(env *tourianv1.Envelope) {
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
		if rec := r.active[m.VelocityCommand.GetRobotId()]; rec != nil {
			rec.lastCommand = m.VelocityCommand
		}
	case *tourianv1.Envelope_Observation:
		if rec := r.active[m.Observation.GetRobotId()]; rec != nil {
			r.observe(rec, m.Observation)
		}
	case *tourianv1.Envelope_VideoFrame:
		if rec := r.active[m.VideoFrame.GetRobotId()]; rec != nil {
			r.picture(rec, m.VideoFrame)
		}
	}
}

func (r *Recorder) start(request *tourianv1.RecordRequest) {
	robotID := request.GetRobotId()
	if _, recording := r.active[robotID]; recording || robotID == "" {
		return
	}
	now := r.now()
	id := now.UTC().Format("20060102-150405") + "-" + safeName(robotID)
	writer, err := NewWriter(filepath.Join(r.root, id))
	if err != nil {
		log.Printf("episode: cannot start %s: %v", id, err)
		return
	}
	rec := &recording{
		episode: &tourianv1.Episode{
			Id:        id,
			RobotId:   robotID,
			Task:      request.GetTask(),
			StartedAt: timestamppb.New(now),
		},
		writer:    writer,
		startedAt: now,
	}
	r.active[robotID] = rec
	r.platform.RequestVideo(robotID, peerID, true)
	r.platform.Broadcast(r.status(rec, true))
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
	if err := rec.writer.Close(rec.episode); err != nil {
		log.Printf("episode: closing %s: %v", rec.episode.GetId(), err)
	}
	r.platform.Broadcast(r.status(rec, false))
	log.Printf("episode: %s ended with %d frames and %d pictures", rec.episode.GetId(), rec.episode.GetFrames(), rec.episode.GetVideoFrames())
}

func (r *Recorder) observe(rec *recording, observation *tourianv1.Observation) {
	frame := &tourianv1.EpisodeFrame{
		Index:         rec.episode.GetFrames(),
		Offset:        durationpb.New(r.offset(rec, observation.GetHeader().GetStamp())),
		Observation:   observation,
		Action:        rec.lastCommand,
		VideoSequence: rec.lastVideo,
	}
	if err := rec.writer.WriteFrame(frame); err != nil {
		log.Printf("episode: writing frame: %v", err)
		return
	}
	rec.episode.Frames++
}

func (r *Recorder) picture(rec *recording, frame *tourianv1.VideoFrame) {
	if err := rec.writer.WriteVideo(frame, r.offset(rec, frame.GetHeader().GetStamp())); err != nil {
		log.Printf("episode: writing picture: %v", err)
		return
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
	dir := filepath.Join(r.root, safeName(flag.GetEpisodeId()))
	episode, err := ReadMeta(dir)
	if err != nil {
		log.Printf("episode: flagging %s: %v", flag.GetEpisodeId(), err)
		return
	}
	episode.Bad = flag.GetBad()
	episode.Note = flag.GetNote()
	if err := WriteMeta(dir, episode); err != nil {
		log.Printf("episode: flagging %s: %v", flag.GetEpisodeId(), err)
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

func safeName(s string) string {
	return unsafe.ReplaceAllString(s, "_")
}
