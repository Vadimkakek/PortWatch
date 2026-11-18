# PortWatch

PortWatch is a lightweight, dependency-free **Go service monitor** for HTTP(S)
and TCP endpoints. It runs simultaneous checks, tracks in-memory uptime and
latency, warns when TLS certificates approach expiry, and exposes a read-only
JSON status page and Prometheus-compatible metrics.

No account, third-party monitoring service, wallet or API key is required.

## Features

- HTTP health checks: 2xx by default, or exact `expect_status`.
- TCP connectivity checks with optional TLS handshake and certificate validation.
- Certificate expiry warning window (`tls_warning_days`, default 14).
- Concurrent probes with an individual timeout per target.
- Session uptime %, consecutive failures, latest latency and state.
- `/status` JSON, `/metrics` for Prometheus, `/healthz` for readiness.
- One-shot checks for CI and scripts (`-once` / `-once -json`).
- **Safe reporting**: outputs contain only user-chosen target IDs, never the
  configured URLs or network addresses.
- Zero third-party Go dependencies.

## Requirements

Go 1.23 or newer.

## Quick start

```bash
git clone https://github.com/Vadimkakek/PortWatch.git
cd PortWatch
cp configs/example.json config.json
# Edit config.json to contain only endpoints you control or are authorized to check.
go run ./cmd/portwatch -config config.json -once
```

The sample uses reserved `.example` domains, so probes will fail until you put
real destinations in your **local ignored** `config.json`. Do not commit this file.

To start continuous monitoring:

```bash
go run ./cmd/portwatch -config config.json
```

By default the monitor listens only on `127.0.0.1:9187`:

```bash
curl http://127.0.0.1:9187/status
curl http://127.0.0.1:9187/metrics
curl http://127.0.0.1:9187/healthz
```

For one JSON snapshot:

```bash
go run ./cmd/portwatch -config config.json -once -json
```

**Note:** `go run` returns a nonzero exit code through Go's wrapper; for exact
exit codes in CI, build and call the binary directly.

```bash
go build -o portwatch ./cmd/portwatch
./portwatch -config config.json -once -json
```

Exit codes: `0` all targets healthy, `1` invalid config/CLI/runtime failure,
`2` one or more unhealthy targets in `-once` mode.

## Local configuration

Example `configs/example.json`:

```json
{
  "interval_seconds": 30,
  "timeout_seconds": 5,
  "tls_warning_days": 14,
  "listen": "127.0.0.1:9187",
  "targets": [
    {"id": "web-health", "kind": "http", "address": "https://status.example/health"},
    {"id": "service-tcp", "kind": "tcp", "address": "service.example:443", "tls": true}
  ]
}
```

For HTTP, `expect_status` can require an exact HTTP response code (e.g., `204`).
Without it, HTTP 2xx is healthy. For TCP, `tls: true` enables a full verified
TLS handshake and certificate expiry reporting.

Allowed IDs use letters, digits, dash and underscore; they must start with a
letter. IDs appear in Prometheus labels and JSON; addresses do not.

## Prometheus example

```text
# HELP portwatch_target_up Whether the target accepted a connection or returned an HTTP response.
# TYPE portwatch_target_up gauge
portwatch_target_up{target="web-health",kind="http"} 1
```

Other series include `portwatch_target_healthy`,
`portwatch_target_latency_seconds`, `portwatch_target_checks_total`,
`portwatch_target_successes_total`, `portwatch_target_consecutive_failures`,
and `portwatch_target_tls_expiry_timestamp_seconds` when a certificate is
available.

`up` describes reachability; `healthy` also accounts for HTTP status and the
configured TLS expiry warning window. Uptime percentage counts **healthy**
checks during the current process lifetime and is reset after restart.

## Privacy and safety

PortWatch does not read private files, keys or wallet material. It sends
network probes only to addresses explicitly configured by the operator.

- Never commit your actual `config.json` or credentials.
- No endpoints, URL paths or raw network errors are published in status/metrics.
- HTTP redirects are not followed and HTTPS/TLS validation is not bypassed.
- Keep the monitor bound to localhost unless you deploy secure access controls.

See [SECURITY.md](SECURITY.md).

## Development

```bash
gofmt -w .
go vet ./...
go test -race ./...
go build ./cmd/portwatch
```

This project intentionally has no third-party dependencies.

## Roadmap

- Persistent uptime storage and historical charts.
- Optional alert notifications on failures and recoveries.
- Histogram latency metrics and SLA/SLO thresholds.
- Docker image and example Prometheus/Grafana dashboards.

## License

MIT
