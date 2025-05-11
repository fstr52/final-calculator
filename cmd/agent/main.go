package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fstr52/final-calculator/internal/agent"
	"github.com/fstr52/final-calculator/internal/config"
	"github.com/fstr52/final-calculator/internal/logger"
)

func main() {
	workCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	tempLogger := logger.NewDefault()

	cfgLoader := config.NewDefaultLoader(tempLogger)
	cfg, err := cfgLoader.Load("agent", "")
	if err != nil {
		tempLogger.Error("Failed to load config",
			"error", err)
		os.Exit(1)
	}

	logger, err := logger.New(&logger.LoggingConfig{
		Format: cfg.Logging.Format,
		LogDir: cfg.Logging.Dir,
		ToFile: cfg.Logging.ToFile,
		Debug:  cfg.Debug,
	})
	if err != nil {
		tempLogger.Error("Failed to create logger",
			"error", err)
		os.Exit(1)
	}

	a := agent.NewAgent(int(cfg.Agent.ComputingPower), cfg.Agent.OrchestratorHost, cfg.Agent.OrchestratorPort, logger)
	go func(agent *agent.Agent) {
		if err := agent.Run(workCtx); err != nil {
			logger.Error("Agent error",
				"error", err)
		}
	}(a)

	<-stop
	logger.Info("Shutdown signal received")

	cancel()

	time.Sleep(2 * time.Second)
	logger.Info("Agent stopped")
}
