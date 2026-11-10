package lerobot

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// A stand-in ffmpeg that writes its last argument, so the orchestration is tested without the real encoder.
func fakeFfmpeg(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nfor last; do :; done\nprintf 'mp4' > \"$last\"\n"
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func jpegPicture(t *testing.T, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 8))
	for y := range 8 {
		for x := range 16 {
			img.Set(x, y, color.RGBA{shade, 0, 255 - shade, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func record(t *testing.T, sink *Sink, id, task string, frames int, x0 float64) *tourianv1.Episode {
	t.Helper()
	meta := &tourianv1.Episode{Id: id, RobotId: "zebes-1", Task: task}
	if err := sink.Begin(meta); err != nil {
		t.Fatal(err)
	}
	for i := range frames {
		offset := time.Duration(i) * 50 * time.Millisecond
		if i%2 == 0 {
			if err := sink.Picture(meta, &tourianv1.VideoFrame{Sequence: uint64(i), Jpeg: jpegPicture(t, uint8(i*20)), Width: 16, Height: 8}, offset); err != nil {
				t.Fatal(err)
			}
		}
		frame := &tourianv1.EpisodeFrame{
			Index:  uint64(i),
			Offset: durationpb.New(offset),
			Observation: &tourianv1.Observation{
				Pose:     &tourianv1.Pose{Position: &tourianv1.Point{X: x0 + float64(i)}, Orientation: &tourianv1.Quaternion{W: 1}},
				Velocity: &tourianv1.Twist{Linear: &tourianv1.Vector3{X: 1}, Angular: &tourianv1.Vector3{}},
				Joints:   &tourianv1.JointState{Name: []string{"wheel_left", "wheel_right"}, Position: []float64{float64(i), -float64(i)}},
				Battery:  &tourianv1.Battery{Percentage: 0.9},
			},
			Action: &tourianv1.VelocityCommand{Twist: &tourianv1.Twist{Linear: &tourianv1.Vector3{X: 0.5}, Angular: &tourianv1.Vector3{Z: 0.1}}},
		}
		if err := sink.Frame(meta, frame); err != nil {
			t.Fatal(err)
		}
	}
	if err := sink.End(meta); err != nil {
		t.Fatal(err)
	}
	return meta
}

func TestWritesTheLayout(t *testing.T) {
	fakeFfmpeg(t)
	root := t.TempDir()
	sink := New(root, 20)
	record(t, sink, "ep-a", "drive a square", 4, 0)
	record(t, sink, "ep-b", "drive a circle", 6, 10)
	record(t, sink, "ep-c", "drive a square", 2, 20)

	var info info
	readJSON(t, filepath.Join(root, "meta/info.json"), &info)
	if info.CodebaseVersion != "v3.0" || info.TotalEpisodes != 3 || info.TotalFrames != 12 || info.TotalTasks != 2 || info.Splits["train"] != "0:3" || info.Fps != 20 {
		t.Errorf("info = %+v", info)
	}
	if info.Features["observation.state"].Shape[0] != 8 || info.Features["observation.images.front"].Dtype != "video" || info.Features["observation.images.front"].Shape[0] != 8 {
		t.Errorf("features = %+v", info.Features)
	}

	tasks, err := parquet.ReadFile[taskRow](filepath.Join(root, "meta/tasks.parquet"))
	if err != nil || len(tasks) != 2 || tasks[0].Task != "drive a square" || tasks[1].TaskIndex != 1 {
		t.Errorf("tasks = %v, %v", tasks, err)
	}

	frames, err := parquet.ReadFile[frameRow](filepath.Join(root, "data/chunk-000/file-001.parquet"))
	if err != nil || len(frames) != 6 {
		t.Fatalf("second episode's frames = %d, %v", len(frames), err)
	}
	last := frames[5]
	if last.EpisodeIndex != 1 || last.FrameIndex != 5 || last.Index != 9 || last.TaskIndex != 1 || last.Timestamp != 0.25 || last.State[0] != 15 || last.State[5] != 5 || last.State[7] != 0.9 || last.Action[1] != 0.1 {
		t.Errorf("last frame = %+v", last)
	}

	episodes, err := parquet.ReadFile[episodeRow](filepath.Join(root, "meta/episodes/chunk-000/file-002.parquet"))
	if err != nil || len(episodes) != 1 {
		t.Fatalf("episodes = %v, %v", episodes, err)
	}
	ep := episodes[0]
	if ep.EpisodeIndex != 2 || ep.Length != 2 || ep.From != 10 || ep.To != 12 || ep.DataFile != 2 || ep.Tasks[0] != "drive a square" || ep.VideoTo == nil || *ep.VideoTo != 0.1 {
		t.Errorf("episode row = %+v", ep)
	}
	if data, err := os.ReadFile(filepath.Join(root, "videos/observation.images.front/chunk-000/file-002.mp4")); err != nil || string(data) != "mp4" {
		t.Errorf("video: %v %q", err, data)
	}

	var stats map[string]*featureStats
	readJSON(t, filepath.Join(root, "meta/stats.json"), &stats)
	state := stats["observation.state"]
	if state.Count[0] != 12 || state.Min[0] != 0 || state.Max[0] != 21 || len(state.Q50) != 8 {
		t.Errorf("state stats = %+v", state)
	}
	if image := stats["observation.images.front"]; image == nil || len(image.Mean) != 3 || image.Max[2] > 1 {
		t.Errorf("image stats = %+v", image)
	}

	if err := sink.Flag("ep-b", true, "slipped"); err != nil {
		t.Fatal(err)
	}
	var mapping map[string]mapping
	readJSON(t, filepath.Join(root, "meta/tourian.json"), &mapping)
	if mapping["ep-b"].EpisodeIndex != 1 || !mapping["ep-b"].Bad || mapping["ep-b"].Note != "slipped" || mapping["ep-a"].Bad {
		t.Errorf("mapping = %+v", mapping)
	}
}

func TestWithoutFfmpegThereIsNoVideo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	record(t, New(root, 20), "ep-a", "look", 3, 0)
	var info info
	readJSON(t, filepath.Join(root, "meta/info.json"), &info)
	if _, ok := info.Features["observation.images.front"]; ok || info.VideoPath != nil {
		t.Errorf("no video feature expected, got %+v", info)
	}
	if _, err := os.Stat(filepath.Join(root, "videos")); !os.IsNotExist(err) {
		t.Error("no videos directory expected")
	}
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatal(err)
	}
}
