// PortWatch monitors explicitly configured HTTP and TCP targets.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Vadimkakek/PortWatch/internal/config"
	"github.com/Vadimkakek/PortWatch/internal/monitor"
	"github.com/Vadimkakek/PortWatch/internal/server"
)

const version = "0.1.0"

func main() {
	os.Exit(run())
}

func run() int {
	flags := flag.NewFlagSet("portwatch", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", "config.json", "path to a local JSON configuration file")
	listen := flags.String("listen", "", "override local metrics listen address")
	once := flags.Bool("once", false, "check all targets once and exit")
	jsonOutput := flags.Bool("json", false, "with -once, print machine-readable JSON")
	showVersion := flags.Bool("version", false, "show version")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return 1
	}
	if *showVersion {
		fmt.Println("PortWatch", version)
		return 0
	}
	if flags.NArg() != 0 || (*jsonOutput && !*once) {
		fmt.Fprintln(os.Stderr, "PortWatch: unexpected arguments, or -json requires -once")
		return 1
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "PortWatch:", err)
		return 1
	}
	if *listen != "" {
		cfg.Listen = *listen
		if err := cfg.Validate(); err != nil {
			fmt.Fprintln(os.Stderr, "PortWatch:", err)
			return 1
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	m := monitor.New(cfg)
	m.RunOnce(ctx)
	if *once {
		snapshot := m.Snapshot()
		if *jsonOutput {
			_ = json.NewEncoder(os.Stdout).Encode(snapshot)
		} else {
			for _, s := range snapshot.Targets {
				fmt.Printf("%-20s %-9s latency=%0.2fms uptime=%0.1f%%\n", s.ID, s.State, s.LatencyMS, s.UptimePercent)
			}
		}
		for _, s := range snapshot.Targets {
			if !s.Healthy {
				return 2
			}
		}
		return 0
	}

	go func() {
		ticker := time.NewTicker(cfg.Interval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.RunOnce(ctx)
			}
		}
	}()

	httpServer := server.NewHTTPServer(cfg.Listen, server.Handler(m))
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	fmt.Println("PortWatch monitoring; endpoints: /healthz /status /metrics")
	err = httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "PortWatch: local metrics server could not start")
		return 1
	}
	return 0
}
