package config

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/proto"
)

type fakePublisher struct{ published []proto.Message }

func (p *fakePublisher) Publish(message proto.Message) { p.published = append(p.published, message) }

func TestVersionsEveryChange(t *testing.T) {
	store := NewStore(t.TempDir(), time.Now)
	first, err := store.PutRobot(&tourianv1.RobotConfig{RobotId: "zebes-1", Name: "Rover"})
	if err != nil || first.GetVersion() != 1 {
		t.Fatalf("first = %v, %v", first, err)
	}
	second, _ := store.PutRobot(&tourianv1.RobotConfig{RobotId: "zebes-1", Name: "Rover, slower", Limits: &tourianv1.VelocityLimits{MaxLinear: 0.3}})
	if second.GetVersion() != 2 || store.LatestRobot("zebes-1").GetName() != "Rover, slower" || len(store.RobotVersions("zebes-1")) != 2 {
		t.Fatalf("second = %v", second)
	}
	if store.LatestRobot("nobody") != nil || len(store.Robots()) != 1 {
		t.Fatal("only the one robot is configured")
	}
	if _, err := store.PutTask(&tourianv1.TaskConfig{TaskId: "bad id"}); err == nil {
		t.Fatal("ids with spaces are refused")
	}
	task, _ := store.PutTask(&tourianv1.TaskConfig{TaskId: "square", Name: "Drive a square"})
	if task.GetVersion() != 1 || store.LatestTask("square").GetName() != "Drive a square" || len(store.Tasks()) != 1 {
		t.Fatalf("task = %v", task)
	}
}

func TestServesAndSaves(t *testing.T) {
	store := NewStore(t.TempDir(), time.Now)
	publisher := &fakePublisher{}
	server := httptest.NewServer(Handler(store, publisher))
	defer server.Close()

	request, _ := http.NewRequest(http.MethodPut, server.URL+"/config/robots/zebes-1", strings.NewReader(`{"name":"Rover","limits":{"maxLinear":0.5}}`))
	response, err := http.DefaultClient.Do(request)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("put: %v %v", err, response.Status)
	}
	_ = response.Body.Close()
	if len(publisher.published) != 1 || publisher.published[0].(*tourianv1.RobotConfig).GetVersion() != 1 {
		t.Fatalf("saving publishes the new version, got %v", publisher.published)
	}

	response, _ = http.Get(server.URL + "/config/robots")
	body, _ := io.ReadAll(response.Body)
	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil || len(list) != 1 || list[0]["robotId"] != "zebes-1" || list[0]["version"] != float64(1) {
		t.Fatalf("list = %s", body)
	}

	request, _ = http.NewRequest(http.MethodPut, server.URL+"/config/tasks/sq", strings.NewReader(`{"name":"Square"}`))
	response, _ = http.DefaultClient.Do(request)
	_ = response.Body.Close()
	response, _ = http.Get(server.URL + "/config/tasks/sq")
	body, _ = io.ReadAll(response.Body)
	if !strings.Contains(string(body), `"version":1`) {
		t.Fatalf("task versions = %s", body)
	}
}
