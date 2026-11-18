// Package probe checks explicit HTTP and TCP targets. It never emits addresses or credentials.
package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/Vadimkakek/PortWatch/internal/config"
)

// Result contains only safe-to-report probe metadata; it never includes the destination.
type Result struct {
	Up          bool
	Healthy     bool
	Latency     time.Duration
	StatusCode  int
	TLSNotAfter *time.Time
	ErrorKind   string
}

// Check makes one outbound request to a configured target, bounded by timeout.
func Check(ctx context.Context, target config.Target, timeout time.Duration, warningDays int, now time.Time) Result {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	var result Result

	switch target.Kind {
	case "http":
		result = checkHTTP(ctx, target)
	case "tcp":
		result = checkTCP(ctx, target)
	default:
		result.ErrorKind = "configuration_error"
	}
	result.Latency = time.Since(start)

	markTLSExpiry(&result, warningDays, now)
	return result
}

func markTLSExpiry(result *Result, warningDays int, now time.Time) {
	if result.TLSNotAfter != nil && result.Up && !result.TLSNotAfter.After(now.Add(time.Duration(warningDays)*24*time.Hour)) {
		result.Healthy = false
		result.ErrorKind = "tls_expiring"
	}
}

func checkHTTP(ctx context.Context, target config.Target) Result {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.Address, nil)
	if err != nil {
		return Result{ErrorKind: "configuration_error"}
	}
	request.Header.Set("User-Agent", "PortWatch/0.1")

	// No redirects: the configured health URL is the only requested destination.
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &http.Transport{
			Proxy:                  nil,
			TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
			MaxResponseHeaderBytes: 1 << 20,
			DisableKeepAlives:      true,
		},
	}
	defer client.CloseIdleConnections()

	response, err := client.Do(request)
	if err != nil {
		kind := "connection_error"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			kind = "timeout"
		} else if parsed, parseErr := url.Parse(target.Address); parseErr == nil && parsed.Scheme == "https" {
			kind = "tls_or_connection_error"
		}
		return Result{ErrorKind: kind}
	}
	defer response.Body.Close()

	result := Result{StatusCode: response.StatusCode}
	if response.TLS != nil && len(response.TLS.PeerCertificates) > 0 {
		notAfter := response.TLS.PeerCertificates[0].NotAfter
		result.TLSNotAfter = &notAfter
	}
	result.Up = true
	if target.ExpectStatus != 0 {
		result.Healthy = response.StatusCode == target.ExpectStatus
	} else {
		result.Healthy = response.StatusCode >= 200 && response.StatusCode < 300
	}
	if !result.Healthy {
		result.ErrorKind = "http_status"
	}
	return result
}

func checkTCP(ctx context.Context, target config.Target) Result {
	var dialer net.Dialer
	connection, err := dialer.DialContext(ctx, "tcp", target.Address)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Result{ErrorKind: "timeout"}
		}
		return Result{ErrorKind: "connection_error"}
	}
	defer connection.Close()

	if !target.TLS {
		return Result{Up: true, Healthy: true}
	}

	host, _, _ := net.SplitHostPort(target.Address) // validated by config.
	tlsConn := tls.Client(connection, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	defer tlsConn.Close()
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return Result{ErrorKind: "tls_error"}
	}
	state := tlsConn.ConnectionState()
	result := Result{Up: true, Healthy: true}
	if len(state.PeerCertificates) > 0 {
		notAfter := state.PeerCertificates[0].NotAfter
		result.TLSNotAfter = &notAfter
	}
	return result
}
