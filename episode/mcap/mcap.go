// Package mcap writes episodes as MCAP files for Foxglove: the contract's own messages as protobuf channels with their schema embedded, and the pictures and poses in Foxglove's JSON schemas so its panels show them without being told how.
package mcap

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	foxmcap "github.com/foxglove/mcap/go/mcap"
	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	observationChannel uint16 = iota + 1
	actionChannel
	controllerChannel
	cameraChannel
	poseChannel
)

const (
	observationSchema uint16 = iota + 1
	actionSchema
	controllerSchema
	cameraSchema
	poseSchema
)

// A Sink is one directory with a file per episode.
type Sink struct {
	root string
	open map[string]*writer
}

type writer struct {
	file       *os.File
	mcap       *foxmcap.Writer
	start      time.Time
	sequence   uint32
	controller string
}

func New(root string) *Sink {
	return &Sink{root: root, open: map[string]*writer{}}
}

func (s *Sink) Begin(episode *tourianv1.Episode) error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(s.root, episode.GetId()+".mcap"))
	if err != nil {
		return err
	}
	w, err := foxmcap.NewWriter(file, &foxmcap.WriterOptions{Chunked: true, Compression: foxmcap.CompressionZSTD, ChunkSize: 4 << 20})
	if err != nil {
		_ = file.Close()
		return err
	}
	if err := w.WriteHeader(&foxmcap.Header{Library: "tourian"}); err != nil {
		return err
	}
	for _, schema := range []*foxmcap.Schema{
		{ID: observationSchema, Name: "tourian.v1.Observation", Encoding: "protobuf", Data: descriptorSet((&tourianv1.Observation{}).ProtoReflect().Descriptor())},
		{ID: actionSchema, Name: "tourian.v1.VelocityCommand", Encoding: "protobuf", Data: descriptorSet((&tourianv1.VelocityCommand{}).ProtoReflect().Descriptor())},
		{ID: controllerSchema, Name: "tourian.v1.Controller", Encoding: "protobuf", Data: descriptorSet((&tourianv1.Controller{}).ProtoReflect().Descriptor())},
		{ID: cameraSchema, Name: "foxglove.CompressedImage", Encoding: "jsonschema", Data: []byte(compressedImageSchema)},
		{ID: poseSchema, Name: "foxglove.PoseInFrame", Encoding: "jsonschema", Data: []byte(poseInFrameSchema)},
	} {
		if err := w.WriteSchema(schema); err != nil {
			return err
		}
	}
	for _, channel := range []*foxmcap.Channel{
		{ID: observationChannel, SchemaID: observationSchema, Topic: "/observation", MessageEncoding: "protobuf"},
		{ID: actionChannel, SchemaID: actionSchema, Topic: "/action", MessageEncoding: "protobuf"},
		{ID: controllerChannel, SchemaID: controllerSchema, Topic: "/controller", MessageEncoding: "protobuf"},
		{ID: cameraChannel, SchemaID: cameraSchema, Topic: "/camera", MessageEncoding: "json"},
		{ID: poseChannel, SchemaID: poseSchema, Topic: "/pose", MessageEncoding: "json"},
	} {
		if err := w.WriteChannel(channel); err != nil {
			return err
		}
	}
	s.open[episode.GetId()] = &writer{file: file, mcap: w, start: episode.GetStartedAt().AsTime()}
	return nil
}

func (s *Sink) Frame(episode *tourianv1.Episode, frame *tourianv1.EpisodeFrame) error {
	w := s.open[episode.GetId()]
	if w == nil {
		return fmt.Errorf("mcap: episode %s not open", episode.GetId())
	}
	at := w.start.Add(frame.GetOffset().AsDuration())
	if err := w.writeProto(observationChannel, at, frame.GetObservation()); err != nil {
		return err
	}
	if frame.GetAction() != nil {
		if err := w.writeProto(actionChannel, at, frame.GetAction()); err != nil {
			return err
		}
	}
	if id := frame.GetController().GetId(); id != w.controller {
		w.controller = id
		if err := w.writeProto(controllerChannel, at, frame.GetController()); err != nil {
			return err
		}
	}
	pose := frame.GetObservation().GetPose()
	return w.writeJSON(poseChannel, at, map[string]any{
		"timestamp": foxTime(at),
		"frame_id":  frame.GetObservation().GetHeader().GetFrameId(),
		"pose": map[string]any{
			"position":    map[string]float64{"x": pose.GetPosition().GetX(), "y": pose.GetPosition().GetY(), "z": pose.GetPosition().GetZ()},
			"orientation": map[string]float64{"x": pose.GetOrientation().GetX(), "y": pose.GetOrientation().GetY(), "z": pose.GetOrientation().GetZ(), "w": pose.GetOrientation().GetW()},
		},
	})
}

func (s *Sink) Picture(episode *tourianv1.Episode, frame *tourianv1.VideoFrame, offset time.Duration) error {
	w := s.open[episode.GetId()]
	if w == nil {
		return fmt.Errorf("mcap: episode %s not open", episode.GetId())
	}
	at := w.start.Add(offset)
	return w.writeJSON(cameraChannel, at, map[string]any{
		"timestamp": foxTime(at),
		"frame_id":  frame.GetHeader().GetFrameId(),
		"data":      base64.StdEncoding.EncodeToString(frame.GetJpeg()),
		"format":    "jpeg",
	})
}

func (s *Sink) End(episode *tourianv1.Episode) error {
	w := s.open[episode.GetId()]
	if w == nil {
		return fmt.Errorf("mcap: episode %s not open", episode.GetId())
	}
	delete(s.open, episode.GetId())
	err := w.mcap.WriteMetadata(&foxmcap.Metadata{Name: "tourian", Metadata: map[string]string{
		"episode_id": episode.GetId(),
		"robot_id":   episode.GetRobotId(),
		"task":       episode.GetTask(),
		"task_id":    episode.GetTaskId(),
		"frames":     fmt.Sprint(episode.GetFrames()),
		"pictures":   fmt.Sprint(episode.GetVideoFrames()),
	}})
	if closeErr := w.mcap.Close(); err == nil {
		err = closeErr
	}
	if closeErr := w.file.Close(); err == nil {
		err = closeErr
	}
	return err
}

// Flag keeps the flag beside the file, since a closed MCAP is not rewritten.
func (s *Sink) Flag(episodeID string, bad bool, note string) error {
	data, _ := json.Marshal(map[string]any{"bad": bad, "note": note})
	return os.WriteFile(filepath.Join(s.root, episodeID+".flag.json"), append(data, '\n'), 0o644)
}

func (w *writer) writeProto(channel uint16, at time.Time, message proto.Message) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	return w.write(channel, at, data)
}

func (w *writer) writeJSON(channel uint16, at time.Time, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return w.write(channel, at, data)
}

func (w *writer) write(channel uint16, at time.Time, data []byte) error {
	w.sequence++
	ns := uint64(at.UnixNano())
	return w.mcap.WriteMessage(&foxmcap.Message{ChannelID: channel, Sequence: w.sequence, LogTime: ns, PublishTime: ns, Data: data})
}

func foxTime(at time.Time) map[string]int64 {
	return map[string]int64{"sec": at.Unix(), "nsec": int64(at.Nanosecond())}
}

// descriptorSet is the message's file and everything it imports, which is what a protobuf schema in MCAP holds.
func descriptorSet(message protoreflect.MessageDescriptor) []byte {
	set := &descriptorpb.FileDescriptorSet{}
	seen := map[string]bool{}
	var add func(file protoreflect.FileDescriptor)
	add = func(file protoreflect.FileDescriptor) {
		if seen[file.Path()] {
			return
		}
		seen[file.Path()] = true
		imports := file.Imports()
		for i := range imports.Len() {
			add(imports.Get(i).FileDescriptor)
		}
		set.File = append(set.File, protodesc.ToFileDescriptorProto(file))
	}
	add(message.ParentFile())
	data, _ := proto.Marshal(set)
	return data
}

const compressedImageSchema = `{"title":"foxglove.CompressedImage","type":"object","properties":{"timestamp":{"type":"object","properties":{"sec":{"type":"integer"},"nsec":{"type":"integer"}}},"frame_id":{"type":"string"},"data":{"type":"string","contentEncoding":"base64"},"format":{"type":"string"}}}`

const poseInFrameSchema = `{"title":"foxglove.PoseInFrame","type":"object","properties":{"timestamp":{"type":"object","properties":{"sec":{"type":"integer"},"nsec":{"type":"integer"}}},"frame_id":{"type":"string"},"pose":{"type":"object","properties":{"position":{"type":"object","properties":{"x":{"type":"number"},"y":{"type":"number"},"z":{"type":"number"}}},"orientation":{"type":"object","properties":{"x":{"type":"number"},"y":{"type":"number"},"z":{"type":"number"},"w":{"type":"number"}}}}}}}`
