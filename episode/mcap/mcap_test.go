package mcap

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	foxmcap "github.com/foxglove/mcap/go/mcap"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestWritesAReadableFile(t *testing.T) {
	root := t.TempDir()
	sink := New(root)
	start := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	episode := &tourianv1.Episode{Id: "ep-1", RobotId: "r1", Task: "look around", StartedAt: timestamppb.New(start)}
	if err := sink.Begin(episode); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		frame := &tourianv1.EpisodeFrame{
			Index:       uint64(i),
			Offset:      durationpb.New(time.Duration(i) * 50 * time.Millisecond),
			Observation: &tourianv1.Observation{RobotId: "r1", Pose: &tourianv1.Pose{Position: &tourianv1.Point{X: float64(i)}, Orientation: &tourianv1.Quaternion{W: 1}}},
			Action:      &tourianv1.VelocityCommand{Twist: &tourianv1.Twist{Linear: &tourianv1.Vector3{X: 0.5}}},
			Controller:  &tourianv1.Controller{Id: "square"},
		}
		if err := sink.Frame(episode, frame); err != nil {
			t.Fatal(err)
		}
	}
	if err := sink.Picture(episode, &tourianv1.VideoFrame{Jpeg: []byte("jpeg"), Header: &tourianv1.Header{FrameId: "camera"}}, 60*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	episode.Frames, episode.VideoFrames = 4, 1
	episode.Bad, episode.Note = true, "blurry"
	if err := sink.End(episode); err != nil {
		t.Fatal(err)
	}
	if err := sink.Flag("../ep-1", true, "blurry"); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(filepath.Join(root, "ep-1.mcap"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	reader, err := foxmcap.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	info, err := reader.Info()
	if err != nil {
		t.Fatal(err)
	}
	if info.Statistics.MessageCount != 4+4+1+4+1 {
		t.Errorf("messages = %d", info.Statistics.MessageCount)
	}
	topics := map[string]string{}
	for _, channel := range info.Channels {
		topics[channel.Topic] = info.Schemas[channel.SchemaID].Name
	}
	if topics["/observation"] != "tourian.v1.Observation" || topics["/camera"] != "foxglove.CompressedImage" || topics["/pose"] != "foxglove.PoseInFrame" {
		t.Errorf("topics = %v", topics)
	}
	if len(info.MetadataIndexes) != 1 || info.MetadataIndexes[0].Name != "tourian" {
		t.Errorf("metadata = %v", info.MetadataIndexes)
	}
	metadata, err := reader.GetMetadata(info.MetadataIndexes[0].Offset)
	if err != nil || metadata.Metadata["bad"] != "true" || metadata.Metadata["note"] != "blurry" {
		t.Errorf("metadata = %v, %v", metadata, err)
	}

	it, err := reader.Messages(foxmcap.WithTopics([]string{"/observation"}))
	if err != nil {
		t.Fatal(err)
	}
	var last *tourianv1.Observation
	for {
		_, _, message, err := it.NextInto(nil)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		last = &tourianv1.Observation{}
		if err := proto.Unmarshal(message.Data, last); err != nil {
			t.Fatal(err)
		}
		if message.LogTime < uint64(start.UnixNano()) {
			t.Errorf("log time before the episode started")
		}
	}
	if last.GetPose().GetPosition().GetX() != 3 {
		t.Errorf("last observation = %v", last)
	}
	if _, err := os.Stat(filepath.Join(root, ".._ep-1.flag.json")); err != nil {
		t.Error("the flag should sit beside the file, under a name kept inside the directory")
	}
}
