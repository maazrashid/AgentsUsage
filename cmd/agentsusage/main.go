package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maazrashid/AgentsUsage/internal/config"
	"github.com/maazrashid/AgentsUsage/internal/parser"
	"github.com/maazrashid/AgentsUsage/internal/service"
	trayui "github.com/maazrashid/AgentsUsage/internal/tray"
)

func main() {
	if err := run(); err != nil {
		log.Printf("AgentsUsage stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	defaultConfig, err := config.DefaultPath()
	if err != nil {
		return err
	}
	configPath := flag.String("config", defaultConfig, "path to config.json")
	forceStart := flag.Bool("start", false, "start the server even when server.autoStart is false")
	noTray := flag.Bool("no-tray", false, "run as a headless server without a system tray icon")
	flag.Parse()

	resolvedConfigPath, err := config.ExpandPath(*configPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	cfg, err := config.Load(resolvedConfigPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	monitor := parser.NewMonitor(parser.ScanOptions{
		ClaudeRoot: cfg.Paths.ClaudeLogs,
		CodexRoot:  cfg.Paths.CodexLogs,
	}, time.Duration(cfg.RefreshIntervalSeconds)*time.Second)
	go func() {
		if err := monitor.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("usage monitor stopped: %v", err)
			stop()
		}
	}()

	controller := service.NewController(cfg.Server.Address(), monitor)
	defer func() {
		if err := controller.StopWithTimeout(); err != nil {
			log.Printf("dashboard server shutdown: %v", err)
		}
	}()
	shouldStart := cfg.Server.AutoStart || *forceStart
	if shouldStart {
		if err := controller.Start(); err != nil {
			if *noTray {
				return err
			}
			log.Printf("dashboard server did not start: %v", err)
		} else {
			log.Printf("AgentsUsage dashboard: %s", cfg.Server.DashboardURL())
		}
	}

	if *noTray {
		if !shouldStart {
			return fmt.Errorf("server auto-start is disabled; run with -start or enable the tray")
		}
		<-ctx.Done()
		return nil
	}

	return trayui.Run(ctx, trayui.Options{
		DashboardURL: cfg.Server.DashboardURL(),
		ConfigPath:   resolvedConfigPath,
		Quit:         stop,
	}, controller, monitor)
}
