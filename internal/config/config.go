// Package config loads local PortWatch configuration without logging sensitive addresses.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	defaultIntervalSeconds = 30
	defaultTimeoutSeconds  = 5
	defaultWarningDays     = 14
	defaultListen          = "127.0.0.1:9187"
	maxTargets             = 100
)

var targetIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// Target is one explicitly configured destination. Address is never included in reports.
type Target struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Address      string `json:"address"`
	TLS          bool   `json:"tls,omitempty"`
	ExpectStatus int    `json:"expect_status,omitempty"`
}

// Config configures probe frequency, deadlines, TLS warnings and local HTTP endpoints.
type Config struct {
	IntervalSeconds int      `json:"interval_seconds"`
	TimeoutSeconds  int      `json:"timeout_seconds"`
	TLSWarningDays  int      `json:"tls_warning_days"`
	Listen          string   `json:"listen"`
	Targets         []Target `json:"targets"`
}

func (c Config) Interval() time.Duration { return time.Duration(c.IntervalSeconds) * time.Second }
func (c Config) Timeout() time.Duration  { return time.Duration(c.TimeoutSeconds) * time.Second }

// Load parses a single JSON document and validates it. Error messages avoid target addresses.
func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, errors.New("cannot open configuration file")
	}
	defer file.Close()

	cfg := Config{
		IntervalSeconds: defaultIntervalSeconds,
		TimeoutSeconds:  defaultTimeoutSeconds,
		TLSWarningDays:  defaultWarningDays,
		Listen:          defaultListen,
	}

	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, errors.New("invalid configuration JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Config{}, errors.New("configuration must contain exactly one JSON object")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks that the configuration is bounded and safe to describe publicly.
func (c Config) Validate() error {
	if c.IntervalSeconds < 1 || c.IntervalSeconds > 86400 {
		return errors.New("interval_seconds must be between 1 and 86400")
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 300 {
		return errors.New("timeout_seconds must be between 1 and 300")
	}
	if c.TLSWarningDays < 0 || c.TLSWarningDays > 3650 {
		return errors.New("tls_warning_days must be between 0 and 3650")
	}
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return errors.New("listen must be a host:port address")
	}
	if len(c.Targets) == 0 || len(c.Targets) > maxTargets {
		return fmt.Errorf("targets must contain between 1 and %d entries", maxTargets)
	}

	seen := make(map[string]bool, len(c.Targets))
	for index, target := range c.Targets {
		if !targetIDPattern.MatchString(target.ID) {
			return fmt.Errorf("target %d: id must be 1-64 ASCII letters, numbers, '-' or '_', starting with a letter", index+1)
		}
		if seen[target.ID] {
			return fmt.Errorf("target %d: duplicate id", index+1)
		}
		seen[target.ID] = true
		switch target.Kind {
		case "http":
			parsed, err := url.Parse(target.Address)
			if err != nil || parsed.Hostname() == "" || parsed.Opaque != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
				return fmt.Errorf("target %d: http address must be an http(s) URL without credentials, query or fragment", index+1)
			}
			if target.TLS {
				return fmt.Errorf("target %d: TLS is automatic for https; tls option is for tcp only", index+1)
			}
			if target.ExpectStatus != 0 && (target.ExpectStatus < 100 || target.ExpectStatus > 599) {
				return fmt.Errorf("target %d: expect_status must be between 100 and 599", index+1)
			}
		case "tcp":
			host, port, err := net.SplitHostPort(target.Address)
			if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
				return fmt.Errorf("target %d: tcp address must be host:port", index+1)
			}
			if target.ExpectStatus != 0 {
				return fmt.Errorf("target %d: expect_status only applies to http", index+1)
			}
		default:
			return fmt.Errorf("target %d: kind must be http or tcp", index+1)
		}
	}
	return nil
}
