package lerobot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

type frameRow struct {
	State        []float32 `parquet:"observation.state,list"`
	Action       []float32 `parquet:"action,list"`
	Timestamp    float32   `parquet:"timestamp"`
	FrameIndex   int64     `parquet:"frame_index"`
	EpisodeIndex int64     `parquet:"episode_index"`
	Index        int64     `parquet:"index"`
	TaskIndex    int64     `parquet:"task_index"`
}

type episodeRow struct {
	EpisodeIndex int64    `parquet:"episode_index"`
	Tasks        []string `parquet:"tasks,list"`
	Length       int64    `parquet:"length"`
	DataChunk    int64    `parquet:"data/chunk_index"`
	DataFile     int64    `parquet:"data/file_index"`
	From         int64    `parquet:"dataset_from_index"`
	To           int64    `parquet:"dataset_to_index"`
	MetaChunk    int64    `parquet:"meta/episodes/chunk_index"`
	MetaFile     int64    `parquet:"meta/episodes/file_index"`
	VideoChunk   *int64   `parquet:"videos/observation.images.front/chunk_index,optional"`
	VideoFile    *int64   `parquet:"videos/observation.images.front/file_index,optional"`
	VideoFrom    *float32 `parquet:"videos/observation.images.front/from_timestamp,optional"`
	VideoTo      *float32 `parquet:"videos/observation.images.front/to_timestamp,optional"`
}

type taskRow struct {
	TaskIndex int64  `parquet:"task_index"`
	Task      string `parquet:"task"`
}

func writeParquet[T any](path string, rows []T, options ...parquet.WriterOption) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := parquet.NewGenericWriter[T](f, append([]parquet.WriterOption{parquet.Compression(&parquet.Snappy)}, options...)...)
	if _, err := writer.Write(rows); err != nil {
		_ = f.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// The task table is read with pandas, which keeps the task text as the index; this metadata is what pandas writes so that it reads it back the same way.
const pandasTasksMetadata = `{"index_columns": ["task"], "column_indexes": [{"name": null, "field_name": null, "pandas_type": "unicode", "numpy_type": "object", "metadata": {"encoding": "UTF-8"}}], "columns": [{"name": "task_index", "field_name": "task_index", "pandas_type": "int64", "numpy_type": "int64", "metadata": null}, {"name": "task", "field_name": "task", "pandas_type": "unicode", "numpy_type": "object", "metadata": null}], "creator": {"library": "pyarrow", "version": "17.0.0"}, "pandas_version": "2.2.2"}`

// ensureTask returns the index of the task, adding it to the table when it is new.
func (s *Sink) ensureTask(task string) (int64, error) {
	path := filepath.Join(s.root, tasksPath)
	var tasks []taskRow
	if _, err := os.Stat(path); err == nil {
		tasks, err = parquet.ReadFile[taskRow](path)
		if err != nil {
			return 0, err
		}
	}
	for _, row := range tasks {
		if row.Task == task {
			return row.TaskIndex, nil
		}
	}
	index := int64(len(tasks))
	tasks = append(tasks, taskRow{TaskIndex: index, Task: task})
	return index, writeParquet(path, tasks, parquet.KeyValueMetadata("pandas", pandasTasksMetadata))
}

type info struct {
	CodebaseVersion  string             `json:"codebase_version"`
	RobotType        string             `json:"robot_type"`
	TotalEpisodes    int64              `json:"total_episodes"`
	TotalFrames      int64              `json:"total_frames"`
	TotalTasks       int64              `json:"total_tasks"`
	ChunksSize       int                `json:"chunks_size"`
	Fps              int                `json:"fps"`
	Splits           map[string]string  `json:"splits"`
	DataPath         string             `json:"data_path"`
	VideoPath        *string            `json:"video_path"`
	DataFilesSizeMB  int                `json:"data_files_size_in_mb"`
	VideoFilesSizeMB int                `json:"video_files_size_in_mb"`
	Features         map[string]feature `json:"features"`
}

type feature struct {
	Dtype string         `json:"dtype"`
	Shape []int          `json:"shape"`
	Names any            `json:"names"`
	Info  map[string]any `json:"info,omitempty"`
}

// loadInfo reads the dataset's schema, or lays it down from the first episode: the state from its joints, the camera from its pictures, the video only when ffmpeg can encode it.
func (s *Sink) loadInfo(ep *episode) (*info, error) {
	path := filepath.Join(s.root, infoPath)
	if data, err := os.ReadFile(path); err == nil {
		var existing info
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, err
		}
		if names, ok := existing.Features[stateKey].Names.([]any); ok {
			strs := make([]string, len(names))
			for i, n := range names {
				strs[i] = fmt.Sprint(n)
			}
			f := existing.Features[stateKey]
			f.Names = strs
			existing.Features[stateKey] = f
		}
		return &existing, nil
	}
	first := ep.frames[0].GetObservation()
	stateNames := StateNames(first.GetJoints().GetName())
	videoPath := "videos/{video_key}/chunk-{chunk_index:03d}/file-{file_index:03d}.mp4"
	created := &info{
		CodebaseVersion:  codebaseVersion,
		RobotType:        ep.meta.GetRobotId(),
		ChunksSize:       chunkSize,
		Fps:              s.fps,
		Splits:           map[string]string{"train": "0:0"},
		DataPath:         "data/chunk-{chunk_index:03d}/file-{file_index:03d}.parquet",
		VideoPath:        &videoPath,
		DataFilesSizeMB:  100,
		VideoFilesSizeMB: 200,
		Features: map[string]feature{
			stateKey:        {Dtype: "float32", Shape: []int{len(stateNames)}, Names: stateNames},
			actionKey:       {Dtype: "float32", Shape: []int{len(actionNames)}, Names: actionNames},
			"timestamp":     {Dtype: "float32", Shape: []int{1}},
			"frame_index":   {Dtype: "int64", Shape: []int{1}},
			"episode_index": {Dtype: "int64", Shape: []int{1}},
			"index":         {Dtype: "int64", Shape: []int{1}},
			"task_index":    {Dtype: "int64", Shape: []int{1}},
		},
	}
	if len(ep.pictures) > 0 && s.ffmpeg != "" {
		p := ep.pictures[0]
		created.Features[videoKey] = feature{
			Dtype: "video",
			Shape: []int{int(p.height), int(p.width), 3},
			Names: []string{"height", "width", "channels"},
			Info: map[string]any{
				"video.fps":          s.fps,
				"video.codec":        "h264",
				"video.pix_fmt":      "yuv420p",
				"video.is_depth_map": false,
				"has_audio":          false,
			},
		}
	} else {
		created.VideoPath = nil
	}
	return created, nil
}

func (s *Sink) writeInfo(i *info) error {
	tasks, _ := parquet.ReadFile[taskRow](filepath.Join(s.root, tasksPath))
	i.TotalTasks = int64(len(tasks))
	return writeJSON(filepath.Join(s.root, infoPath), i)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
