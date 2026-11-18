// Package server exposes local health, JSON status, and Prometheus-compatible metrics.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Vadimkakek/PortWatch/internal/monitor"
)

// Handler serves read-only endpoints. Raw configured addresses are never included.
func Handler(m *monitor.Monitor) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(m.Snapshot())
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(Metrics(m.Snapshot())))
	})
	return mux
}

// Metrics returns Prometheus text format with only validated target IDs as labels.
func Metrics(snapshot monitor.Snapshot) string {
	var b strings.Builder
	types := []struct{ name, help string }{
		{"portwatch_target_up", "Whether the target accepted a connection or returned an HTTP response."},
		{"portwatch_target_healthy", "Whether the latest check passed all configured conditions."},
		{"portwatch_target_latency_seconds", "Duration of the most recent check in seconds."},
		{"portwatch_target_checks_total", "Number of checks since this monitor started."},
		{"portwatch_target_successes_total", "Number of healthy checks since this monitor started."},
		{"portwatch_target_consecutive_failures", "Current consecutive unhealthy check count."},
		{"portwatch_target_tls_expiry_timestamp_seconds", "TLS certificate expiration as a Unix timestamp, when available."},
	}
	for _, metric := range types {
		metricType := "gauge"
		if strings.HasSuffix(metric.name, "_total") {
			metricType = "counter"
		}
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", metric.name, metric.help, metric.name, metricType)
		for _, s := range snapshot.Targets {
			labels := fmt.Sprintf("{target=%q,kind=%q}", s.ID, s.Kind)
			switch metric.name {
			case "portwatch_target_up":
				fmt.Fprintf(&b, "%s%s %d\n", metric.name, labels, boolInt(s.Up))
			case "portwatch_target_healthy":
				fmt.Fprintf(&b, "%s%s %d\n", metric.name, labels, boolInt(s.Healthy))
			case "portwatch_target_latency_seconds":
				fmt.Fprintf(&b, "%s%s %.6f\n", metric.name, labels, s.LatencyMS/1000)
			case "portwatch_target_checks_total":
				fmt.Fprintf(&b, "%s%s %d\n", metric.name, labels, s.Checks)
			case "portwatch_target_successes_total":
				fmt.Fprintf(&b, "%s%s %d\n", metric.name, labels, s.Successes)
			case "portwatch_target_consecutive_failures":
				fmt.Fprintf(&b, "%s%s %d\n", metric.name, labels, s.ConsecutiveFailures)
			case "portwatch_target_tls_expiry_timestamp_seconds":
				if s.TLSExpiresAt != nil {
					fmt.Fprintf(&b, "%s%s %d\n", metric.name, labels, s.TLSExpiresAt.Unix())
				}
			}
		}
	}
	return b.String()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func NewHTTPServer(listen string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
