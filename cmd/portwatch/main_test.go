package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestOneShotExitCodes(t *testing.T) {
	var code atomic.Int64
	code.Store(204)
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(int(code.Load()))
	}))
	defer endpoint.Close()

	path := filepath.Join(t.TempDir(), "config.json")
	content := fmt.Sprintf(`{"targets":[{"id":"site","kind":"http","address":%q}]}`, endpoint.URL)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	os.Args = []string{"portwatch", "-config", path, "-once", "-json"}
	if exit := run(); exit != 0 {
		t.Fatalf("healthy target exit=%d, want 0", exit)
	}
	code.Store(503)
	if exit := run(); exit != 2 {
		t.Fatalf("unhealthy target exit=%d, want 2", exit)
	}
}

func TestInvalidConfigurationReturnsError(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	os.Args = []string{"portwatch", "-config", filepath.Join(t.TempDir(), "missing.json"), "-once"}
	if exit := run(); exit != 1 {
		t.Fatalf("missing config exit=%d, want 1", exit)
	}
}
