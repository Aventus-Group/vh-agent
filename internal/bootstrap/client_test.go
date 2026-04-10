package bootstrap

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Heartbeat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bootstrap/heartbeat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("auth header = %q, want Bearer test-token", auth)
		}
		var req HeartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.ContainerID != "c-1" {
			t.Errorf("container_id = %q", req.ContainerID)
		}
		resp := ConfigResponse{
			WG: WGConfig{
				RouterPublicKey: "pubkey",
				RouterEndpoint:  "1.2.3.4:48720",
				AssignedIP:      "10.10.75.42",
			},
			Agent: AgentConfig{
				LatestVersion: "1.0.0",
				DownloadURL:   "https://x/agent",
				SHA256:        "abc",
			},
			HeartbeatIntervalSec: 300,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", 2*time.Second)
	req := &HeartbeatRequest{ContainerID: "c-1", AgentVersion: "1.0.0"}

	resp, err := c.Heartbeat(req)
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if resp.WG.RouterEndpoint != "1.2.3.4:48720" {
		t.Errorf("RouterEndpoint = %q", resp.WG.RouterEndpoint)
	}
	if resp.HeartbeatIntervalSec != 300 {
		t.Errorf("HeartbeatIntervalSec = %d", resp.HeartbeatIntervalSec)
	}
}

func TestClient_Heartbeat_401ReturnsErrUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad-token", 2*time.Second)
	_, err := c.Heartbeat(&HeartbeatRequest{ContainerID: "c-1"})
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("error = %v, want ErrUnauthorized", err)
	}
}

func TestClient_Heartbeat_5xxReturnsRetryableError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", 2*time.Second)
	_, err := c.Heartbeat(&HeartbeatRequest{ContainerID: "c-1"})
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrUnauthorized) {
		t.Errorf("5xx should not be ErrUnauthorized")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention 500: %v", err)
	}
}

func TestClient_GetConfig_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bootstrap/config/c-1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		resp := ConfigResponse{
			WG:                   WGConfig{RouterEndpoint: "2.3.4.5:48720"},
			HeartbeatIntervalSec: 300,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-token", 2*time.Second)
	resp, err := c.GetConfig("c-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.WG.RouterEndpoint != "2.3.4.5:48720" {
		t.Errorf("endpoint = %q", resp.WG.RouterEndpoint)
	}
}
