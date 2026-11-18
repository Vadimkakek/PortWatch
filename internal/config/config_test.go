package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfig() Config {
	return Config{
		IntervalSeconds: 30,
		TimeoutSeconds:  5,
		TLSWarningDays:  14,
		Listen:          "127.0.0.1:9187",
		Targets: []Target{
			{ID: "site", Kind: "http", Address: "https://status.example/health"},
			{ID: "tcp-one", Kind: "tcp", Address: "service.example:443", TLS: true},
		},
	}
}

func TestLoadDefaultsAndUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"targets":[{"id":"site","kind":"http","address":"https://status.example/health"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IntervalSeconds != 30 || cfg.TimeoutSeconds != 5 || cfg.Listen != "127.0.0.1:9187" {
		t.Fatalf("defaults not applied: %+v", cfg)
	}

	if err := os.WriteFile(path, []byte(`{"targets":[],"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestValidateRejectsPrivateURLPartsAndBadID(t *testing.T) {
	for _, input := range []string{
		strings.Join([]string{"https://user:pass", "@", "status.example/health"}, ""),
		"https://status.example/health?query=invalid",
		"https://status.example/health#private",
	} {
		cfg := validConfig()
		cfg.Targets[0].Address = input
		err := cfg.Validate()
		if err == nil {
			t.Fatalf("expected rejection for sensitive URL")
		}
		if strings.Contains(err.Error(), input) || strings.Contains(err.Error(), "pass") || strings.Contains(err.Error(), "invalid") {
			t.Fatalf("error echoed sensitive input: %v", err)
		}
	}

	cfg := validConfig()
	cfg.Targets[0].ID = "private host"
	if cfg.Validate() == nil {
		t.Fatal("expected invalid ID rejection")
	}
}

func TestValidateRejectsDuplicateAndInvalidTarget(t *testing.T) {
	cfg := validConfig()
	cfg.Targets[1].ID = cfg.Targets[0].ID
	if cfg.Validate() == nil {
		t.Fatal("expected duplicate rejection")
	}
	cfg = validConfig()
	cfg.Targets[1].Kind = "ssh"
	if cfg.Validate() == nil {
		t.Fatal("expected invalid kind rejection")
	}
	cfg = validConfig()
	cfg.Targets[1].ExpectStatus = 200
	if cfg.Validate() == nil {
		t.Fatal("expected tcp status rejection")
	}
	cfg = validConfig()
	cfg.Targets[0].TLS = true
	if cfg.Validate() == nil {
		t.Fatal("expected ambiguous TLS configuration rejection")
	}
}

func TestValidateRejectsInvalidBounds(t *testing.T) {
	cfg := validConfig()
	cfg.TimeoutSeconds = 0
	if cfg.Validate() == nil {
		t.Fatal("expected bad timeout rejection")
	}
	cfg = validConfig()
	cfg.Listen = "0.0.0.0"
	if cfg.Validate() == nil {
		t.Fatal("expected invalid listen rejection")
	}
}
