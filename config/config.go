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
	s.mu.Lock()
	defer s.mu.Unlock()
	version, err := s.nextVersion("robots", c.GetRobotId())
	if err != nil {
		return nil, err
	}
	stored := proto.Clone(c).(*tourianv1.RobotConfig)
	stored.Version = version
	stored.UpdatedAt = timestamppb.New(s.now())
	return stored, s.write("robots", c.GetRobotId(), version, stored)
}

func (s *Store) PutTask(c *tourianv1.TaskConfig) (*tourianv1.TaskConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	version, err := s.nextVersion("tasks", c.GetTaskId())
	if err != nil {
		return nil, err
	}
	stored := proto.Clone(c).(*tourianv1.TaskConfig)
	stored.Version = version
	stored.UpdatedAt = timestamppb.New(s.now())
	return stored, s.write("tasks", c.GetTaskId(), version, stored)
}

// LatestRobot is the robot's current configuration, nil when it has none.
func (s *Store) LatestRobot(robotID string) *tourianv1.RobotConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	versions := s.versions("robots", robotID)
	if len(versions) == 0 {
		return nil
	}
	c := &tourianv1.RobotConfig{}
	if s.read("robots", robotID, versions[len(versions)-1], c) != nil {
		return nil
	}
	return c
}

func (s *Store) LatestTask(taskID string) *tourianv1.TaskConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	versions := s.versions("tasks", taskID)
	if len(versions) == 0 {
		return nil
	}
	c := &tourianv1.TaskConfig{}
	if s.read("tasks", taskID, versions[len(versions)-1], c) != nil {
		return nil
	}
	return c
}

// Robots lists the current configuration of every robot that has one, by id.
func (s *Store) Robots() []*tourianv1.RobotConfig {
	var out []*tourianv1.RobotConfig
	for _, id := range s.ids("robots") {
		if c := s.LatestRobot(id); c != nil {
			out = append(out, c)
		}
	}
	return out
}

func (s *Store) Tasks() []*tourianv1.TaskConfig {
	var out []*tourianv1.TaskConfig
	for _, id := range s.ids("tasks") {
		if c := s.LatestTask(id); c != nil {
			out = append(out, c)
		}
	}
	return out
}

// RobotVersions is every version of a robot's configuration, oldest first.
func (s *Store) RobotVersions(robotID string) []*tourianv1.RobotConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*tourianv1.RobotConfig
	for _, v := range s.versions("robots", robotID) {
		c := &tourianv1.RobotConfig{}
		if s.read("robots", robotID, v, c) == nil {
			out = append(out, c)
		}
	}
	return out
}

func (s *Store) TaskVersions(taskID string) []*tourianv1.TaskConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*tourianv1.TaskConfig
	for _, v := range s.versions("tasks", taskID) {
		c := &tourianv1.TaskConfig{}
		if s.read("tasks", taskID, v, c) == nil {
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
