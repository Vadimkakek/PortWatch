package probe

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Vadimkakek/PortWatch/internal/config"
)

func TestHTTPHealthyAndStatusError(t *testing.T) {
	var code atomic.Int64
	code.Store(204)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(int(code.Load()))
	}))
	defer server.Close()
	target := config.Target{ID: "site", Kind: "http", Address: server.URL}

	good := Check(context.Background(), target, time.Second, 14, time.Now())
	if !good.Up || !good.Healthy || good.StatusCode != 204 || good.Latency < 0 {
		t.Fatalf("unexpected result: %+v", good)
	}
	code.Store(503)
	bad := Check(context.Background(), target, time.Second, 14, time.Now())
	if !bad.Up || bad.Healthy || bad.ErrorKind != "http_status" {
		t.Fatalf("unexpected degraded result: %+v", bad)
	}
	target.ExpectStatus = 503
	expected := Check(context.Background(), target, time.Second, 14, time.Now())
	if !expected.Healthy {
		t.Fatalf("expected status override failed: %+v", expected)
	}
}

func TestHTTPDoesNotFollowRedirect(t *testing.T) {
	var followed atomic.Bool
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		followed.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer final.Close()
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	defer first.Close()

	result := Check(context.Background(), config.Target{ID: "site", Kind: "http", Address: first.URL}, time.Second, 14, time.Now())
	if result.Healthy || result.StatusCode != 302 || followed.Load() {
		t.Fatalf("redirect must not be followed: result=%+v followed=%v", result, followed.Load())
	}
}

func TestTCPConnectivityAndClosedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	target := config.Target{ID: "tcp", Kind: "tcp", Address: address}
	good := Check(context.Background(), target, time.Second, 14, time.Now())
	if !good.Up || !good.Healthy {
		t.Fatalf("expected TCP to be healthy: %+v", good)
	}
	listener.Close()

	bad := Check(context.Background(), target, time.Second, 14, time.Now())
	if bad.Up || bad.Healthy || bad.ErrorKind != "connection_error" {
		t.Fatalf("expected connection failure: %+v", bad)
	}
}

func TestHTTPSUntrustedCertificateFailsClosed(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	result := Check(context.Background(), config.Target{ID: "self-signed", Kind: "http", Address: server.URL}, 2*time.Second, 14, time.Now())
	if result.Up || result.Healthy || result.ErrorKind != "tls_or_connection_error" {
		t.Fatalf("expected untrusted TLS to be rejected: %+v", result)
	}
}

func TestHTTPTimeoutReportsOnlyGenericError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	result := Check(context.Background(), config.Target{ID: "slow", Kind: "http", Address: server.URL}, 20*time.Millisecond, 14, time.Now())
	if result.Up || result.Healthy || result.ErrorKind != "timeout" {
		t.Fatalf("expected timeout: %+v", result)
	}
}

func TestTLSWarningMarksUpButUnhealthy(t *testing.T) {
	now := time.Now()
	certificateExpiry := now.Add(24 * time.Hour)
	result := Result{Up: true, Healthy: true, TLSNotAfter: &certificateExpiry}
	markTLSExpiry(&result, 14, now)
	if !result.Up || result.Healthy || result.ErrorKind != "tls_expiring" {
		t.Fatalf("expected expiry warning: %+v", result)
	}
}
