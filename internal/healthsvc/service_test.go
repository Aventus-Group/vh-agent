package healthsvc

import (
	"context"
	"testing"
	"time"

	"github.com/Aventus-Group/vh-agent/gen/agentpb"
)

func TestHealth_ReportsVersionAndUptime(t *testing.T) {
	svc := New("v0.1.0", time.Now().Add(-3*time.Second))
	resp, err := svc.Health(context.Background(), &agentpb.HealthRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Version != "v0.1.0" {
		t.Errorf("version = %q", resp.Version)
	}
	if resp.UptimeSec < 2 || resp.UptimeSec > 5 {
		t.Errorf("uptime_sec = %d (want ~3)", resp.UptimeSec)
	}
}
