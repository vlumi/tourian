// Package lerobot writes episodes in the LeRobot dataset layout, version 3.0: Parquet tables for the frames and the episodes, Parquet for the tasks, JSON for the schema and the statistics, and one MP4 per episode for the camera when ffmpeg is at hand.
package lerobot

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	episodes "github.com/vlumi/tourian/episode"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
)

const (
	codebaseVersion = "v3.0"
	chunkSize       = 1000
	videoKey        = "observation.images.front"
	stateKey        = "observation.state"
	actionKey       = "action"
	infoPath        = "meta/info.json"
	statsPath       = "meta/stats.json"
	tasksPath       = "meta/tasks.parquet"
	episodesPattern = "meta/episodes/chunk-%03d/file-%03d.parquet"
	dataPattern     = "data/chunk-%03d/file-%03d.parquet"
	videoPattern    = "videos/%s/chunk-%03d/file-%03d.mp4"
	mappingPath     = "meta/tourian.json"
)

var actionNames = []string{"linear_velocity", "angular_velocity"}

// A Sink is one dataset directory per robot under its root, since a dataset's state vector is laid out from one robot's joints. Each episode becomes its own data file, episodes file and video file, so nothing is rewritten when one ends.
type Sink struct {
	root   string
	fps    int
	ffmpeg string
	open   map[string]*episode
}

type dataset struct {
	root string
}

type episode struct {
	meta     *tourianv1.Episode
	frames   []*tourianv1.EpisodeFrame
	pictures []picture
	dir      string
}

type picture struct {
	offset time.Duration
	file   string
	width  uint32
	height uint32
}

func New(root string, fps int) *Sink {
	ffmpeg, _ := exec.LookPath("ffmpeg")
	return &Sink{root: root, fps: fps, ffmpeg: ffmpeg, open: map[string]*episode{}}
}

func (s *Sink) Begin(meta *tourianv1.Episode) error {
	dir, err := os.MkdirTemp("", "tourian-lerobot-")
	if err != nil {
		return err
	}
	s.open[meta.GetId()] = &episode{meta: meta, dir: dir}
	return nil
}

func (s *Sink) Frame(meta *tourianv1.Episode, frame *tourianv1.EpisodeFrame) error {
	ep := s.open[meta.GetId()]
	if ep == nil {
		return errors.New("lerobot: episode not open")
	}
	ep.frames = append(ep.frames, frame)
	return nil
}

func (s *Sink) Picture(meta *tourianv1.Episode, frame *tourianv1.VideoFrame, offset time.Duration) error {
	ep := s.open[meta.GetId()]
	if ep == nil {
		return errors.New("lerobot: episode not open")
	}
	file := filepath.Join(ep.dir, fmt.Sprintf("%06d.jpg", len(ep.pictures)))
	if err := os.WriteFile(file, frame.GetJpeg(), 0o644); err != nil {
		return err
	}
	ep.pictures = append(ep.pictures, picture{offset: offset, file: file, width: frame.GetWidth(), height: frame.GetHeight()})
	return nil
}

// End lays the episode down: the frames as a data file, a row in an episodes file, the task in the task table, the statistics folded into the dataset's, the video encoded, and the schema's totals advanced.
func (s *Sink) End(meta *tourianv1.Episode) error {
	ep := s.open[meta.GetId()]
	if ep == nil {
		return errors.New("lerobot: episode not open")
	}
	delete(s.open, meta.GetId())
	defer func() { _ = os.RemoveAll(ep.dir) }()
	if len(ep.frames) == 0 {
		return nil
	}

	d := s.dataset(meta.GetRobotId())
	info, err := d.loadInfo(s, ep)
	if err != nil {
		return err
	}
	index := info.TotalEpisodes
	chunk, file := int(index/chunkSize), int(index%chunkSize)
	from := info.TotalFrames
	length := int64(len(ep.frames))
	stateNames := info.Features[stateKey].Names

	taskIndex, tasks, err := d.ensureTask(meta.GetTask())
	if err != nil {
		return err
	}
	info.TotalTasks = tasks

	rows := make([]frameRow, 0, len(ep.frames))
	for i, frame := range ep.frames {
		rows = append(rows, frameRow{
			State:        stateVector(frame.GetObservation(), stateNames),
			Action:       actionVector(frame.GetAction()),
			Timestamp:    float32(i) / float32(s.fps),
			FrameIndex:   int64(i),
			EpisodeIndex: index,
			Index:        from + int64(i),
			TaskIndex:    taskIndex,
		})
	}
	if err := writeParquet(filepath.Join(d.root, fmt.Sprintf(dataPattern, chunk, file)), rows); err != nil {
		return err
	}

	row := episodeRow{
		EpisodeIndex: index,
		Tasks:        []string{meta.GetTask()},
		Length:       length,
		DataChunk:    int64(chunk),
		DataFile:     int64(file),
		From:         from,
		To:           from + length,
		MetaChunk:    int64(chunk),
		MetaFile:     int64(file),
	}
	_, hasVideo := info.Features[videoKey]
	if hasVideo && len(ep.pictures) > 0 && s.ffmpeg != "" {
		if err := s.encodeVideo(ep, filepath.Join(d.root, fmt.Sprintf(videoPattern, videoKey, chunk, file))); err != nil {
			return err
		}
		to := float32(length) / float32(s.fps)
		zero := float32(0)
		c, f := int64(chunk), int64(file)
		row.VideoChunk, row.VideoFile, row.VideoFrom, row.VideoTo = &c, &f, &zero, &to
	}
	if err := writeParquet(filepath.Join(d.root, fmt.Sprintf(episodesPattern, chunk, file)), []episodeRow{row}); err != nil {
		return err
	}

	stats := map[string]*featureStats{
		stateKey:  vectorStats(rows, func(r frameRow) []float32 { return r.State }),
		actionKey: vectorStats(rows, func(r frameRow) []float32 { return r.Action }),
	}
	if hasVideo && len(ep.pictures) > 0 {
		if imageStats, err := pictureStats(ep.pictures, int(length)); err == nil {
			stats[videoKey] = imageStats
		}
	}
	if err := d.foldStats(stats); err != nil {
		return err
	}

	info.TotalEpisodes++
	info.TotalFrames += length
	info.Splits = map[string]string{"train": fmt.Sprintf("0:%d", info.TotalEpisodes)}
	if err := d.writeInfo(info); err != nil {
		return err
	}
	return d.updateMapping(func(m map[string]mapping) {
		m[meta.GetId()] = mapping{EpisodeIndex: index, Bad: meta.GetBad(), Note: meta.GetNote()}
	})
}

func (s *Sink) dataset(robotID string) dataset {
	return dataset{root: filepath.Join(s.root, episodes.SafeName(robotID))}
}

// Flag keeps the flag beside the dataset, since the layout has no place for it; tourian.json maps episode ids to indices as well. The dataset is found by the robot the id names, which is how the recorder names them.
func (s *Sink) Flag(episodeID string, bad bool, note string) error {
	for _, d := range s.datasets() {
		var found bool
		err := d.updateMapping(func(m map[string]mapping) {
			entry, ok := m[episodeID]
			if !ok {
				return
			}
			found = true
			entry.Bad = bad
			entry.Note = note
			m[episodeID] = entry
		})
		if err != nil || found {
			return err
		}
	}
	return nil
}

func (s *Sink) datasets() []dataset {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil
	}
	var out []dataset
	for _, entry := range entries {
		if entry.IsDir() {
			out = append(out, dataset{root: filepath.Join(s.root, entry.Name())})
		}
	}
	return out
}

type mapping struct {
	EpisodeIndex int64  `json:"episode_index"`
	Bad          bool   `json:"bad"`
	Note         string `json:"note,omitempty"`
}

func (d dataset) updateMapping(change func(map[string]mapping)) error {
	path := filepath.Join(d.root, mappingPath)
	m := map[string]mapping{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
	}
	change(m)
	return writeJSON(path, m)
}

func stateVector(observation *tourianv1.Observation, names []string) []float32 {
	pose := observation.GetPose()
	yaw := 2 * math.Atan2(pose.GetOrientation().GetZ(), pose.GetOrientation().GetW())
	state := []float32{
		float32(pose.GetPosition().GetX()),
		float32(pose.GetPosition().GetY()),
		float32(yaw),
		float32(observation.GetVelocity().GetLinear().GetX()),
		float32(observation.GetVelocity().GetAngular().GetZ()),
	}
	joints := observation.GetJoints()
	for i := range len(names) - len(state) - 1 {
		var position float64
		if i < len(joints.GetPosition()) {
			position = joints.GetPosition()[i]
		}
		state = append(state, float32(position))
	}
	return append(state, float32(observation.GetBattery().GetPercentage()))
}

func actionVector(action *tourianv1.VelocityCommand) []float32 {
	return []float32{
		float32(action.GetTwist().GetLinear().GetX()),
		float32(action.GetTwist().GetAngular().GetZ()),
	}
}

// StateNames is the layout of observation.state: pose, body velocity, one entry per joint, and the battery.
func StateNames(jointNames []string) []string {
	names := []string{"x", "y", "yaw", "linear_velocity", "angular_velocity"}
	names = append(names, jointNames...)
	return append(names, "battery")
}

// The video is the picture in force at each frame, so it runs at the dataset's rate and lines up with the rows.
func (s *Sink) encodeVideo(ep *episode, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var list strings.Builder
	current := 0
	for i, frame := range ep.frames {
		at := frame.GetOffset().AsDuration()
		for current+1 < len(ep.pictures) && ep.pictures[current+1].offset <= at {
			current++
		}
		fmt.Fprintf(&list, "file '%s'\nduration %.6f\n", ep.pictures[current].file, 1/float64(s.fps))
		if i == len(ep.frames)-1 {
			fmt.Fprintf(&list, "file '%s'\n", ep.pictures[current].file)
		}
	}
	listPath := filepath.Join(ep.dir, "frames.txt")
	if err := os.WriteFile(listPath, []byte(list.String()), 0o644); err != nil {
		return err
	}
	cmd := exec.Command(s.ffmpeg, "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", listPath,
		"-vf", "format=yuv420p", "-r", fmt.Sprint(s.fps), "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("lerobot: ffmpeg: %w: %s", err, out)
	}
	return nil
}

func sortedCopy(values []float64) []float64 {
	out := append([]float64(nil), values...)
	sort.Float64s(out)
	return out
}
