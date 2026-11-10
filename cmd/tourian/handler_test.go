package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vlumi/tourian/hub"
)

func TestHealthz(t *testing.T) {
	server := httptest.NewServer(newHandler(hub.New(), t.TempDir()))
	defer server.Close()

	response, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}
