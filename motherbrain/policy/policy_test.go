package policy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestAsksTheServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		observation := &tourianv1.Observation{}
		if err := protojson.Unmarshal(body, observation); err != nil || r.URL.Path != "/act" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		twist := &tourianv1.Twist{Linear: &tourianv1.Vector3{X: observation.GetPose().GetPosition().GetX() / 2}}
		data, _ := protojson.Marshal(twist)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := New(server.URL)
	twist, err := client.Act(context.Background(), &tourianv1.Observation{Pose: &tourianv1.Pose{Position: &tourianv1.Point{X: 3}}})
	if err != nil || twist.GetLinear().GetX() != 1.5 {
		t.Fatalf("twist = %v, err = %v", twist, err)
	}
	if client.Describe().GetKind() != tourianv1.ControllerKind_CONTROLLER_KIND_POLICY {
		t.Error("kind")
	}

	down := New("http://127.0.0.1:1")
	if _, err := down.Act(context.Background(), &tourianv1.Observation{}); err == nil {
		t.Fatal("a server that cannot be reached is an error, not a twist")
	}
}
