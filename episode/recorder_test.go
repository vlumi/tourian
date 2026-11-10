package episode

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/transport/ws"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakePlatform struct {
	broadcasts []proto.Message
	requests   []string
}

func (p *fakePlatform) Broadcast(message proto.Message) { p.broadcasts = append(p.broadcasts, message) }
func (p *fakePlatform) RequestVideo(robotID, peerID string, enabled bool) {
	state := "off"
	if enabled {
		state = "on"
	}
	p.requests = append(p.requests, robotID+":"+peerID+":"+state)
}

func wrap(t *testing.T, message proto.Message) *tourianv1.Envelope {
	t.Helper()
	env, err := ws.Wrap(message)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestRecordsAnEpisode(t *testing.T) {
	root := t.TempDir()
	platform := &fakePlatform{}
	clock := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	recorder := NewRecorder(platform, func() time.Time { return clock }, NewJSONL(root))

	recorder.Handle(wrap(t, &tourianv1.RecordRequest{RobotId: "r1", Start: true, Task: "drive a square"}))
	if len(platform.requests) != 1 || platform.requests[0] != "r1:recorder:on" {
		t.Fatalf("video should be requested on start, got %v", platform.requests)
	}
	status := platform.broadcasts[0].(*tourianv1.EpisodeStatus)
	if !status.GetRecording() || status.GetEpisodeId() != "20261110-120000-r1" {
		t.Fatalf("status = %v", status)
	}

	recorder.Handle(wrap(t, &tourianv1.VelocityCommand{RobotId: "r1", Sequence: 5, Twist: &tourianv1.Twist{Linear: &tourianv1.Vector3{X: 0.5}}}))
	recorder.Handle(wrap(t, &tourianv1.Observation{RobotId: "other", Header: stamp(clock)}))
	for i := range 3 {
		at := clock.Add(time.Duration(i) * 50 * time.Millisecond)
		recorder.Handle(wrap(t, &tourianv1.Observation{RobotId: "r1", Header: stamp(at), Pose: &tourianv1.Pose{Position: &tourianv1.Point{X: float64(i)}}}))
		if i == 1 {
			recorder.Handle(wrap(t, &tourianv1.VideoFrame{RobotId: "r1", Header: stamp(at), Sequence: 42, Jpeg: []byte("jpeg"), Width: 4, Height: 3}))
		}
	}
	clock = clock.Add(time.Second)
	recorder.Handle(wrap(t, &tourianv1.RecordRequest{RobotId: "r1", Start: false}))
	if platform.requests[1] != "r1:recorder:off" {
		t.Fatalf("video should be released on stop, got %v", platform.requests)
	}

	dir := filepath.Join(root, "20261110-120000-r1")
	episode, err := ReadMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	if episode.GetFrames() != 3 || episode.GetVideoFrames() != 1 || episode.GetTask() != "drive a square" || episode.GetEndedAt().AsTime().Sub(episode.GetStartedAt().AsTime()) != time.Second {
		t.Errorf("episode = %v", episode)
	}

	frames := readFrames(t, filepath.Join(dir, framesFile))
	if len(frames) != 3 {
		t.Fatalf("frames = %d", len(frames))
	}
	if frames[2].GetIndex() != 2 || frames[2].GetOffset().AsDuration() != 100*time.Millisecond || frames[2].GetVideoSequence() != 42 || frames[2].GetAction().GetSequence() != 5 {
		t.Errorf("last frame = %v", frames[2])
	}
	if frames[0].GetVideoSequence() != 0 {
		t.Errorf("the first frame should have no picture yet, got %d", frames[0].GetVideoSequence())
	}
	if data, err := os.ReadFile(filepath.Join(dir, videoDir, "000042.jpg")); err != nil || string(data) != "jpeg" {
		t.Errorf("picture file: %v, %q", err, data)
	}

	recorder.Handle(wrap(t, &tourianv1.EpisodeFlag{EpisodeId: episode.GetId(), Bad: true, Note: "hit the wall"}))
	flagged, _ := ReadMeta(dir)
	if !flagged.GetBad() || flagged.GetNote() != "hit the wall" {
		t.Errorf("flagged = %v", flagged)
	}

	listed, err := List(root)
	if err != nil || len(listed) != 1 || listed[0].GetId() != episode.GetId() {
		t.Errorf("list = %v, %v", listed, err)
	}
}

func TestIgnoresWhatIsNotRecording(t *testing.T) {
	recorder := NewRecorder(&fakePlatform{}, time.Now, NewJSONL(t.TempDir()))
	recorder.Handle(wrap(t, &tourianv1.Observation{RobotId: "r1"}))
	recorder.Handle(wrap(t, &tourianv1.RecordRequest{RobotId: "r1", Start: false}))
	if len(recorder.active) != 0 {
		t.Fatal("nothing should be recording")
	}
}

func stamp(at time.Time) *tourianv1.Header {
	return &tourianv1.Header{Stamp: timestamppb.New(at)}
}

func readFrames(t *testing.T, path string) []*tourianv1.EpisodeFrame {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var frames []*tourianv1.EpisodeFrame
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		frame := &tourianv1.EpisodeFrame{}
		if err := protojson.Unmarshal(scanner.Bytes(), frame); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	return frames
}
