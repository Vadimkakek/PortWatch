package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Vadimkakek/PortWatch/internal/config"
	"github.com/Vadimkakek/PortWatch/internal/monitor"
)

func TestHTTPStatusAndMetricsNeverShowDestination(t *testing.T) {
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer endpoint.Close()
	cfg := config.Config{IntervalSeconds: 30, TimeoutSeconds: 1, TLSWarningDays: 14, Listen: "127.0.0.1:9187", Targets: []config.Target{{ID: "website", Kind: "http", Address: endpoint.URL + "/internal/secret-path"}}}
	m := monitor.New(cfg)
	m.RunOnce(context.Background())
	server := httptest.NewServer(Handler(m))
	defer server.Close()

	for _, path := range []string{"/status", "/metrics"} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if strings.Contains(text, "secret-path") || strings.Contains(text, endpoint.URL) || strings.Contains(text, "127.0.0.1") {
			t.Fatalf("%s leaked destination: %s", path, text)
		}
		if !strings.Contains(text, "website") {
			t.Fatalf("%s missing safe target ID", path)
		}
	}
}

func TestMetricsContainExpectedSeries(t *testing.T) {
	text := Metrics(monitor.Snapshot{Targets: []monitor.Status{{ID: "site", Kind: "http", Up: true, Healthy: true, Checks: 2, Successes: 2}}})
	for _, want := range []string{
		`portwatch_target_up{target="site",kind="http"} 1`,
		`portwatch_target_healthy{target="site",kind="http"} 1`,
		`portwatch_target_checks_total{target="site",kind="http"} 2`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing metric %q", want)
		}
	}
}
