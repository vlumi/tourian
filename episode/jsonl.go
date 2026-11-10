// Package episode records what a robot saw and was told, frame by frame, into a dataset on disk.
package episode

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	metaFile   = "meta.json"
	framesFile = "frames.jsonl"
	videoFile  = "video.jsonl"
	videoDir   = "video"
)

// JSONL lays each episode out in its own directory for reading by eye and by anything: one JSON object per line for the frames and the video index, the pictures as JPEG files, and the metadata as a JSON file written when the episode ends.
type JSONL struct {
	root    string
	writers map[string]*Writer
}

func NewJSONL(root string) *JSONL {
	return &JSONL{root: root, writers: map[string]*Writer{}}
}

func (s *JSONL) Begin(episode *tourianv1.Episode) error {
	writer, err := NewWriter(filepath.Join(s.root, SafeName(episode.GetId())))
	if err != nil {
		return err
	}
	s.writers[episode.GetId()] = writer
	return nil
}

func (s *JSONL) Frame(episode *tourianv1.Episode, frame *tourianv1.EpisodeFrame) error {
	return s.writers[episode.GetId()].WriteFrame(frame)
}

func (s *JSONL) Picture(episode *tourianv1.Episode, frame *tourianv1.VideoFrame, offset time.Duration) error {
	return s.writers[episode.GetId()].WriteVideo(frame, offset)
}

func (s *JSONL) End(episode *tourianv1.Episode) error {
	writer := s.writers[episode.GetId()]
	delete(s.writers, episode.GetId())
	return writer.Close(episode)
}

func (s *JSONL) Flag(episodeID string, bad bool, note string) error {
	dir := filepath.Join(s.root, SafeName(episodeID))
	episode, err := ReadMeta(dir)
	if err != nil {
		return err
	}
	episode.Bad = bad
	episode.Note = note
	return WriteMeta(dir, episode)
}

// A Writer is one episode under way in the JSONL layout.
type Writer struct {
	dir    string
	frames *os.File
	video  *os.File
}

func NewWriter(dir string) (*Writer, error) {
	if err := os.MkdirAll(filepath.Join(dir, videoDir), 0o755); err != nil {
		return nil, err
	}
	frames, err := os.Create(filepath.Join(dir, framesFile))
	if err != nil {
		return nil, err
	}
	video, err := os.Create(filepath.Join(dir, videoFile))
	if err != nil {
		_ = frames.Close()
		return nil, err
	}
	return &Writer{dir: dir, frames: frames, video: video}, nil
}

func (w *Writer) WriteFrame(frame *tourianv1.EpisodeFrame) error {
	return writeLine(w.frames, frame)
}

func (w *Writer) WriteVideo(frame *tourianv1.VideoFrame, offset time.Duration) error {
	file := filepath.Join(videoDir, fmt.Sprintf("%06d.jpg", frame.GetSequence()))
	if err := os.WriteFile(filepath.Join(w.dir, file), frame.GetJpeg(), 0o644); err != nil {
		return err
	}
	return writeLine(w.video, &tourianv1.EpisodeVideo{
		Sequence: frame.GetSequence(),
		Offset:   durationpb.New(offset),
		File:     filepath.ToSlash(file),
		Width:    frame.GetWidth(),
		Height:   frame.GetHeight(),
	})
}

func (w *Writer) Close(episode *tourianv1.Episode) error {
	if err := w.frames.Close(); err != nil {
		return err
	}
	if err := w.video.Close(); err != nil {
		return err
	}
	return WriteMeta(w.dir, episode)
}

func writeLine(f *os.File, message proto.Message) error {
	data, err := protojson.Marshal(message)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

func WriteMeta(dir string, episode *tourianv1.Episode) error {
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(episode)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, metaFile), append(data, '\n'), 0o644)
}

func ReadMeta(dir string) (*tourianv1.Episode, error) {
	data, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		return nil, err
	}
	episode := &tourianv1.Episode{}
	if err := protojson.Unmarshal(data, episode); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, metaFile), err)
	}
	return episode, nil
}

// List reads the metadata of every episode under root, newest first.
func List(root string) ([]*tourianv1.Episode, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var episodes []*tourianv1.Episode
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() {
			continue
		}
		episode, err := ReadMeta(filepath.Join(root, entries[i].Name()))
		if err != nil {
			continue
		}
		episodes = append(episodes, episode)
	}
	return episodes, nil
}
