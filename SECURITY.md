# Security and privacy

PortWatch only probes destinations explicitly configured in a **local** JSON file.
It does not require any wallet, seed phrase, private key, API token or RPC key.

- **Never commit real monitoring destinations or credentials.** The default
  `config.json`, `config.*.json`, `*.local.json`, `.env*`, and secret configs are
  git-ignored. The included configuration uses reserved `.example` domains.
- Only validated target IDs appear in JSON, Prometheus labels, and CLI output.
  Full destination URLs and addresses are not published by the server.
- HTTP credentials, URL query strings and fragments are not accepted.
- Redirects are disabled. TLS certificate validation remains enabled.
- Metrics bind to loopback (`127.0.0.1`) by default. If you bind to a public
  interface, secure it with your own authentication, TLS, and network access rules.
- This tool **actively sends network checks** to configured destinations;
  it does not fetch any destinations automatically from a remote service.
- Uptime statistics are in-memory and reset when the process restarts.
- Logs do not contain raw error messages from destinations.

If you report a security issue, please do not publish secrets or private hostnames.
