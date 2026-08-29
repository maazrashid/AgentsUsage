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
	"github.com/maazrashid/AgentsUsage/internal/server"
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
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if !cfg.Server.AutoStart && !*forceStart {
		return fmt.Errorf("server auto-start is disabled; run with -start or update %s", *configPath)
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

	httpServer := server.New(cfg.Server.Address(), monitor)
	log.Printf("AgentsUsage dashboard: http://localhost:%d", cfg.Server.Port)
	return httpServer.Run(ctx)
}
