package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/vlumi/tourian/config"
	"github.com/vlumi/tourian/hub"
)

func TestHealthz(t *testing.T) {
	server := httptest.NewServer(newHandler(hub.New(), t.TempDir(), config.NewStore(t.TempDir(), time.Now), nil, "", reach{LAN: "http://lab:8080", Secure: "https://lab:8443"}))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	addresses, err := http.Get(server.URL + "/api/addresses")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = addresses.Body.Close() }()
	body, _ := io.ReadAll(addresses.Body)
	if string(body) != "{\"lan\":\"http://lab:8080\",\"secure\":\"https://lab:8443\"}\n" {
		t.Fatalf("addresses = %s", body)
	}
}

func TestTheConsoleIsServedBehindThePlatform(t *testing.T) {
	console := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("console " + r.URL.Path + " from " + r.Host + " origin " + r.Header.Get("Origin")))
	}))
	defer console.Close()
	target, _ := url.Parse(console.URL)
	server := httptest.NewServer(newHandler(hub.New(), t.TempDir(), config.NewStore(t.TempDir(), time.Now), target, t.TempDir(), reach{}))
	defer server.Close()

	request, _ := http.NewRequest(http.MethodGet, server.URL+"/camera?robot=r1", nil)
	request.Header.Set("Origin", "https://laptop:8443")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, _ := io.ReadAll(response.Body)
	if want := "console /camera from " + target.Host + " origin "; string(body) != want {
		t.Fatalf("got %q, want %q", body, want)
	}
	if response, err := http.Get(server.URL + "/api/healthz"); err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("the platform's own endpoints come first, got %v %v", response, err)
	}
}
