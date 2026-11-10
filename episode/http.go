package episode

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"google.golang.org/protobuf/encoding/protojson"
)

var (
	episodeID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	pictureID = regexp.MustCompile(`^[0-9]{6}\.jpg$`)
)

// Handler serves the episodes under root for browsing: the list, one episode's metadata, its frames and video index as they are on disk, and its pictures.
func Handler(root string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /episodes", func(w http.ResponseWriter, _ *http.Request) {
		episodes, err := List(root)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var body bytes.Buffer
		body.WriteByte('[')
		for i, episode := range episodes {
			if i > 0 {
				body.WriteByte(',')
			}
			data, _ := protojson.Marshal(episode)
			body.Write(data)
		}
		body.WriteByte(']')
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body.Bytes())
	})
	mux.HandleFunc("GET /episodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, root, r.PathValue("id"), metaFile, "application/json")
	})
	mux.HandleFunc("GET /episodes/{id}/frames", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, root, r.PathValue("id"), framesFile, "application/x-ndjson")
	})
	mux.HandleFunc("GET /episodes/{id}/video", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, root, r.PathValue("id"), videoFile, "application/x-ndjson")
	})
	mux.HandleFunc("GET /episodes/{id}/video/{picture}", func(w http.ResponseWriter, r *http.Request) {
		picture := r.PathValue("picture")
		if !pictureID.MatchString(picture) {
			http.NotFound(w, r)
			return
		}
		serveFile(w, r, root, r.PathValue("id"), filepath.Join(videoDir, picture), "image/jpeg")
	})
	return mux
}

func serveFile(w http.ResponseWriter, r *http.Request, root, id, name, contentType string) {
	if !episodeID.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(root, id, name)
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, path)
}
