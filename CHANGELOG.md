# Changelog

## v0.1.0

- Concurrent HTTP/HTTPS and TCP health checks with configurable intervals/timeouts.
- Optional authenticated TLS verification for TCP and TLS certificate expiry warnings.
- Live `/status` JSON and Prometheus-compatible `/metrics` on loopback.
- Safe target ID labels, no destination URLs in reports.
- Session uptime %, request latency, current state, consecutive failures.
- CLI one-shot mode (`-once`, `-json`) with monitoring-friendly exit codes.
- Standard Go library only, unit/integration tests, CI and versioned releases.
