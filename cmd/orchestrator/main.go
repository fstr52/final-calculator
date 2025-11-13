package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fstr52/final-calculator/internal/config"
	"github.com/fstr52/final-calculator/internal/db/postgresql"
	"github.com/fstr52/final-calculator/internal/logger"
	"github.com/fstr52/final-calculator/internal/orchestrator"
	pr "github.com/fstr52/final-calculator/internal/proto"
	"google.golang.org/grpc"
)

func main() {
	workCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	tempLogger := logger.NewDefault()

	cfgLoader := config.NewDefaultLoader(tempLogger)
	cfg, err := cfgLoader.Load("orchestrator", "")
	if err != nil {
		tempLogger.Error("Failed to load config",
			"error", err)
		os.Exit(1)
	}

	logger, err := logger.New(logger.LoggingConfig{
		Format: cfg.Logging.Format,
		LogDir: cfg.Logging.Dir,
		ToFile: cfg.Logging.ToFile,
		Debug:  cfg.Debug,
	})
	if err != nil {
		tempLogger.Error("Failed to create logger",
			"error", err)
	}

	sc := postgresql.StorageConfig{
		Username:   cfg.DataBase.Username,
		Password:   cfg.DataBase.Password,
		Host:       cfg.DataBase.Host,
		Port:       cfg.DataBase.Port,
		Database:   cfg.DataBase.DbName,
		MaxAttemps: cfg.DataBase.MaxAttemps,
	}

	pgClient, err := postgresql.NewClient(workCtx, sc)
	if err != nil {
		logger.Error("Failed to connect to DB",
			"error", err)
	}

	if err := postgresql.InitSchema(workCtx, pgClient); err != nil {
		logger.Error("Failed to init schema",
			"error", err)
	}

	auth, err := orchestrator.NewAuthHander(orchestrator.AuthHandlerConfig{
		Client:     pgClient,
		SigningKey: cfg.SigningKey,
		Logger:     logger,
	})
	if err != nil {
		logger.Error("Failed to create auth handler",
			"error", err)
	}

	oc := orchestrator.OperationsConfig{
		TimeAdditionMS:        cfg.Orchestrator.TimeAdditionMS,
		TimeSubtractionMS:     cfg.Orchestrator.TimeSubtractionMS,
		TimeMultiplicationsMS: cfg.Orchestrator.TimeMultiplicationsMS,
		TimeDivisionsMS:       cfg.Orchestrator.TimeDivisionsMS,
	}

	o := orchestrator.NewOrchestrator(logger, pgClient, oc)
	grpcServer := grpc.NewServer()
	pr.RegisterOrchestratorServiceServer(grpcServer, o)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Error("Failed to listev",
			"error", err)
	}
	logger.Info("gRPC server is listening in :50051")
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed grpcServer.Servce",
				"error", err)
			cancel()
		}
	}()

	router := orchestrator.SetupRouter(o, auth)
	port := ":" + strconv.Itoa(cfg.Orchestrator.Port)

	srv := &http.Server{
		Addr:    port,
		Handler: router,
	}

	go func() {
		logger.Info("Starting server on port",
			"port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error",
				"error", err)
			cancel()
		}
	}()

	go o.Run(workCtx)

	<-stop
	logger.Info("Shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer shutdownCancel()

	cancel()

	logger.Info("Shutting down server...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown failed",
			"error", err)
	} else {
		logger.Info("Server shutdown gracefully")
	}

	<-shutdownCtx.Done()
	logger.Info("App stopped")
}
