// Package monitor runs concurrent checks and maintains in-memory uptime counters.
package monitor

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/Vadimkakek/PortWatch/internal/config"
	"github.com/Vadimkakek/PortWatch/internal/probe"
)

// Status is safe for a public metrics/status response: addresses and secrets are omitted.
type Status struct {
	ID                  string     `json:"id"`
	Kind                string     `json:"kind"`
	State               string     `json:"state"`
	Up                  bool       `json:"up"`
	Healthy             bool       `json:"healthy"`
	LatencyMS           float64    `json:"latency_ms"`
	HTTPStatus          int        `json:"http_status,omitempty"`
	TLSExpiresAt        *time.Time `json:"tls_expires_at,omitempty"`
	Checks              uint64     `json:"checks"`
	Successes           uint64     `json:"successes"`
	UptimePercent       float64    `json:"uptime_percent"`
	ConsecutiveFailures uint64     `json:"consecutive_failures"`
	LastChecked         time.Time  `json:"last_checked"`
	ErrorKind           string     `json:"error_kind,omitempty"`
}

// Snapshot is a point-in-time copy that does not expose configured network addresses.
type Snapshot struct {
	GeneratedAt time.Time `json:"generated_at"`
	Targets     []Status  `json:"targets"`
}

type checker func(context.Context, config.Target, time.Duration, int, time.Time) probe.Result

// Monitor probes targets concurrently, then publishes a synchronized snapshot.
type Monitor struct {
	cfg    config.Config
	check  checker
	clock  func() time.Time
	mu     sync.RWMutex
	states map[string]Status
}

func New(cfg config.Config) *Monitor {
	return &Monitor{
		cfg:    cfg,
		check:  probe.Check,
		clock:  time.Now,
		states: make(map[string]Status, len(cfg.Targets)),
	}
}

// RunOnce probes every target concurrently. Each target has its own timeout.
func (m *Monitor) RunOnce(ctx context.Context) {
	var wg sync.WaitGroup
	for _, target := range m.cfg.Targets {
		wg.Add(1)
		go func(target config.Target) {
			defer wg.Done()
			now := m.clock()
			result := m.check(ctx, target, m.cfg.Timeout(), m.cfg.TLSWarningDays, now)
			status := Status{
				ID:           target.ID,
				Kind:         target.Kind,
				Up:           result.Up,
				Healthy:      result.Healthy,
				LatencyMS:    float64(result.Latency.Microseconds()) / 1000,
				HTTPStatus:   result.StatusCode,
				TLSExpiresAt: result.TLSNotAfter,
				LastChecked:  now,
				ErrorKind:    result.ErrorKind,
			}
			switch {
			case !result.Up:
				status.State = "down"
			case !result.Healthy:
				status.State = "degraded"
			default:
				status.State = "healthy"
			}
			m.mu.Lock()
			previous := m.states[target.ID]
			status.Checks = previous.Checks + 1
			status.Successes = previous.Successes
			if result.Healthy {
				status.Successes++
				status.ConsecutiveFailures = 0
			} else {
				status.ConsecutiveFailures = previous.ConsecutiveFailures + 1
			}
			status.UptimePercent = 100 * float64(status.Successes) / float64(status.Checks)
			m.states[target.ID] = status
			m.mu.Unlock()
		}(target)
	}
	wg.Wait()
}

// Snapshot returns current counters sorted by target ID for stable output.
func (m *Monitor) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := Snapshot{GeneratedAt: m.clock(), Targets: make([]Status, 0, len(m.states))}
	for _, status := range m.states {
		result.Targets = append(result.Targets, status)
	}
	sort.Slice(result.Targets, func(i, j int) bool { return result.Targets[i].ID < result.Targets[j].ID })
	return result
}
