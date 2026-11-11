package config

import (
	"errors"
	"io"
	"net/http"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"github.com/vlumi/tourian/transport/httpjson"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Publisher is told of every new robot configuration, so consoles learn it as it is saved.
type Publisher interface {
	Publish(message proto.Message)
}

// Handler serves the configurations: the current ones as lists, every version of one, and saving a new version with PUT.
func Handler(store *Store, publisher Publisher) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /config/robots", func(w http.ResponseWriter, _ *http.Request) {
		httpjson.WriteList(w, store.Robots())
	})
	mux.HandleFunc("GET /config/robots/{id}", func(w http.ResponseWriter, r *http.Request) {
		httpjson.WriteList(w, store.RobotVersions(r.PathValue("id")))
	})
	mux.HandleFunc("PUT /config/robots/{id}", func(w http.ResponseWriter, r *http.Request) {
		c := &tourianv1.RobotConfig{}
		if !readBody(w, r, c) {
			return
		}
		c.RobotId = r.PathValue("id")
		stored, err := store.PutRobot(c)
		if !ok(w, err) {
			return
		}
		if publisher != nil {
			publisher.Publish(stored)
		}
		httpjson.WriteOne(w, stored)
	})
	mux.HandleFunc("GET /config/tasks", func(w http.ResponseWriter, _ *http.Request) {
		httpjson.WriteList(w, store.Tasks())
	})
	mux.HandleFunc("GET /config/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		httpjson.WriteList(w, store.TaskVersions(r.PathValue("id")))
	})
	mux.HandleFunc("PUT /config/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		c := &tourianv1.TaskConfig{}
		if !readBody(w, r, c) {
			return
		}
		c.TaskId = r.PathValue("id")
		stored, err := store.PutTask(c)
		if !ok(w, err) {
			return
		}
		httpjson.WriteOne(w, stored)
	})
	return mux
}

func readBody(w http.ResponseWriter, r *http.Request, into proto.Message) bool {
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err == nil {
		err = protojson.Unmarshal(data, into)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func ok(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, ErrInvalidID):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return false
}
