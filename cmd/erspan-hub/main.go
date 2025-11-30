package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"anthonyuk.dev/erspan-hub/internal/capture"
	"anthonyuk.dev/erspan-hub/internal/config"
	"anthonyuk.dev/erspan-hub/internal/grpc"
	"anthonyuk.dev/erspan-hub/internal/rest"

	"github.com/spf13/pflag"
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

	if cfg.ShowVersion {
		fmt.Printf("erspan-hub - ERSPAN packet capture hub\n")
		fmt.Printf("Version=%s\n", Version)
		fmt.Printf("Commit=%s\n", Commit)
		fmt.Printf("Date=%s\n", Date)
		buildInfo, ok := debug.ReadBuildInfo()
		if ok {
			fmt.Printf("buildinf: %+v\n", buildInfo)
		}
		os.Exit(0)
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
		rest.RunServer(&rest.Config{BindIP: cfg.RestIP, Port: cfg.RestPort, RestPrefix: cfg.RestPrefix}, ci.ForwardSessionManager())
	}()
	go func() {
		err := grpc.RunServer(&grpc.Config{BindIP: cfg.GrpcIP, Port: cfg.GrpcPort, TLSCertFile: cfg.GrpcTLSCertFile, TLSKeyFile: cfg.GrpcTLSKeyFile}, ci.ForwardSessionManager())
		if err != nil {
			logger.Error("failed to start gRPC server", "error", err)
			panic(err)
		}
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
