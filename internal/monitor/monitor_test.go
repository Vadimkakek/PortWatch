package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Vadimkakek/PortWatch/internal/config"
)

func TestCountersAndSessionUptime(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if healthy.Load() {
			w.WriteHeader(204)
		} else {
			w.WriteHeader(503)
		}
	}))
	defer srv.Close()
	cfg := config.Config{IntervalSeconds: 30, TimeoutSeconds: 1, TLSWarningDays: 14, Listen: "127.0.0.1:9187", Targets: []config.Target{{ID: "status", Kind: "http", Address: srv.URL}}}
	m := New(cfg)
	m.RunOnce(context.Background())
	healthy.Store(false)
	m.RunOnce(context.Background())

	snapshot := m.Snapshot()
	if len(snapshot.Targets) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	s := snapshot.Targets[0]
	if s.Checks != 2 || s.Successes != 1 || s.UptimePercent != 50 || s.ConsecutiveFailures != 1 || s.State != "degraded" {
		t.Fatalf("unexpected uptime state: %+v", s)
	}
}
