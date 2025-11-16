package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/pflag"

	"anthonyuk.dev/erspan-hub/internal/capture"
	"anthonyuk.dev/erspan-hub/internal/config"
	"anthonyuk.dev/erspan-hub/internal/grpc"
	"anthonyuk.dev/erspan-hub/internal/rest"
)

// Override these variables at build time using -ldflags
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		if err == pflag.ErrHelp {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}
	logHandlerOptions := slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	switch cfg.LogLevel {
	case 0:
		logHandlerOptions.Level = slog.LevelWarn
	case 1:
		logHandlerOptions.Level = slog.LevelInfo
	case 2:
		logHandlerOptions.Level = slog.LevelDebug
	default:
		logHandlerOptions.AddSource = true
		logHandlerOptions.Level = slog.LevelDebug
	}
	var logger *slog.Logger
	if cfg.LogJson {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &logHandlerOptions))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &logHandlerOptions))
	}

	server(cfg, logger)
}

func server(cfg *config.Config, logger *slog.Logger) {
	ci := capture.NewCaptureInstance(logger)
	go func() {
		rest.RunServer(&rest.Config{Port: cfg.RestPort}, ci.ForwardSessionManager())
	}()
	go func() {
		grpc.RunServer(&grpc.Config{Port: cfg.GrpcPort}, ci.ForwardSessionManager())
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start packet capture loop
	go func() {
		if err := ci.StartPacketCapture(); err != nil {
			logger.Error("failed to start packet capture", "error", err)
			panic(err)
		}
	}()

	<-quit
	logger.Info("🛑 Stopping capture instance...")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	ci.Shutdown()
	<-ctx.Done()
}
