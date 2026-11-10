package episode

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
)

func TestServesEpisodes(t *testing.T) {
	root := t.TempDir()
	writer, err := NewWriter(filepath.Join(root, "ep-1"))
	if err != nil {
		t.Fatal(err)
	}
	_ = writer.WriteFrame(&tourianv1.EpisodeFrame{Index: 0})
	_ = writer.WriteFrame(&tourianv1.EpisodeFrame{Index: 1})
	_ = writer.WriteVideo(&tourianv1.VideoFrame{Sequence: 7, Jpeg: []byte("jpeg")}, time.Second)
	_ = writer.Close(&tourianv1.Episode{Id: "ep-1", RobotId: "r1", Frames: 2, VideoFrames: 1})

	server := httptest.NewServer(Handler(root))
	defer server.Close()

	var list []map[string]any
	if err := json.Unmarshal(get(t, server.URL+"/episodes", http.StatusOK), &list); err != nil || len(list) != 1 || list[0]["id"] != "ep-1" {
		t.Fatalf("list = %v, %v", list, err)
	}
	if body := get(t, server.URL+"/episodes/ep-1/frames", http.StatusOK); len(body) == 0 {
		t.Fatal("frames should be served")
	}
	if body := get(t, server.URL+"/episodes/ep-1/video", http.StatusOK); len(body) == 0 {
		t.Fatal("video index should be served")
	}
	if body := get(t, server.URL+"/episodes/ep-1/video/000007.jpg", http.StatusOK); string(body) != "jpeg" {
		t.Fatalf("picture = %q", body)
	}
	get(t, server.URL+"/episodes/ep-1/video/..%2Fmeta.json", http.StatusNotFound)
	get(t, server.URL+"/episodes/nope", http.StatusNotFound)
}

func get(t *testing.T, url string, want int) []byte {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != want {
		t.Fatalf("GET %s = %d, want %d", url, response.StatusCode, want)
	}
	body, _ := io.ReadAll(response.Body)
	return body
}
