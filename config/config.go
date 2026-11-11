// Package config keeps the versioned configurations of robots and tasks on disk: every change is a new version written beside the earlier ones, so an episode can name the version it was recorded on and the file is still there to read.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var validID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

var ErrInvalidID = errors.New("config: id must be letters, digits, dots, dashes or underscores")

// A Store is one directory with a subdirectory per kind, a directory per id in it, and a file per version.
type Store struct {
	mu   sync.Mutex
	root string
	now  func() time.Time
}

func NewStore(root string, now func() time.Time) *Store {
	return &Store{root: root, now: now}
}

// PutRobot stores the configuration as the robot's next version and returns it as stored.
func (s *Store) PutRobot(c *tourianv1.RobotConfig) (*tourianv1.RobotConfig, error) {
	return put(s, robots, c.GetRobotId(), c, func(stored *tourianv1.RobotConfig, version uint32, at *timestamppb.Timestamp) {
		stored.Version, stored.UpdatedAt = version, at
	})
}

func (s *Store) PutTask(c *tourianv1.TaskConfig) (*tourianv1.TaskConfig, error) {
	return put(s, tasks, c.GetTaskId(), c, func(stored *tourianv1.TaskConfig, version uint32, at *timestamppb.Timestamp) {
		stored.Version, stored.UpdatedAt = version, at
	})
}

// LatestRobot is the robot's current configuration, nil when it has none.
func (s *Store) LatestRobot(robotID string) *tourianv1.RobotConfig {
	return latest[tourianv1.RobotConfig](s, robots, robotID)
}

func (s *Store) LatestTask(taskID string) *tourianv1.TaskConfig {
	return latest[tourianv1.TaskConfig](s, tasks, taskID)
}

// Robots lists the current configuration of every robot that has one, by id.
func (s *Store) Robots() []*tourianv1.RobotConfig {
	return all[tourianv1.RobotConfig](s, robots)
}

func (s *Store) Tasks() []*tourianv1.TaskConfig {
	return all[tourianv1.TaskConfig](s, tasks)
}

// RobotVersions is every version of a robot's configuration, oldest first.
func (s *Store) RobotVersions(robotID string) []*tourianv1.RobotConfig {
	return history[tourianv1.RobotConfig](s, robots, robotID)
}

func (s *Store) TaskVersions(taskID string) []*tourianv1.TaskConfig {
	return history[tourianv1.TaskConfig](s, tasks, taskID)
}

const (
	robots = "robots"
	tasks  = "tasks"
)

// A config is a generated message reached through its pointer, so that a fresh one can be made from the type alone.
type config[M any] interface {
	*M
	proto.Message
}

func put[M any, C config[M]](s *Store, kind, id string, c C, stamp func(stored C, version uint32, at *timestamppb.Timestamp)) (C, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	version, err := s.nextVersion(kind, id)
	if err != nil {
		return nil, err
	}
	stored := proto.Clone(c).(C)
	stamp(stored, version, timestamppb.New(s.now()))
	return stored, s.write(kind, id, version, stored)
}

func latest[M any, C config[M]](s *Store, kind, id string) C {
	s.mu.Lock()
	defer s.mu.Unlock()
	versions := s.versions(kind, id)
	if len(versions) == 0 {
		return nil
	}
	c := C(new(M))
	if s.read(kind, id, versions[len(versions)-1], c) != nil {
		return nil
	}
	return c
}

func all[M any, C config[M]](s *Store, kind string) []C {
	var out []C
	for _, id := range s.ids(kind) {
		if c := latest[M, C](s, kind, id); c != nil {
			out = append(out, c)
		}
	}
	return out
}

func history[M any, C config[M]](s *Store, kind, id string) []C {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []C
	for _, v := range s.versions(kind, id) {
		c := C(new(M))
		if s.read(kind, id, v, c) == nil {
			out = append(out, c)
		}
	}
	return out
}

func (s *Store) nextVersion(kind, id string) (uint32, error) {
	if !validID.MatchString(id) {
		return 0, ErrInvalidID
	}
	versions := s.versions(kind, id)
	if len(versions) == 0 {
		return 1, nil
	}
	return versions[len(versions)-1] + 1, nil
}

func (s *Store) versions(kind, id string) []uint32 {
	if !validID.MatchString(id) {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(s.root, kind, id))
	if err != nil {
		return nil
	}
	var versions []uint32
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "v") || !strings.HasSuffix(name, ".json") {
			continue
		}
		if v, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(name, "v"), ".json"), 10, 32); err == nil {
			versions = append(versions, uint32(v))
		}
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	return versions
}

func (s *Store) ids(kind string) []string {
	entries, err := os.ReadDir(filepath.Join(s.root, kind))
	if err != nil {
		return nil
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	return ids
}

func (s *Store) path(kind, id string, version uint32) string {
	return filepath.Join(s.root, kind, id, fmt.Sprintf("v%d.json", version))
}

func (s *Store) write(kind, id string, version uint32, message proto.Message) error {
	path := s.path(kind, id, version)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(message)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func (s *Store) read(kind, id string, version uint32, into proto.Message) error {
	data, err := os.ReadFile(s.path(kind, id, version))
	if err != nil {
		return err
	}
	return protojson.Unmarshal(data, into)
}
