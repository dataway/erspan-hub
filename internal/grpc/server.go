package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pcap_v1 "anthonyuk.dev/erspan-hub/generated/pcap/v1"
	streams_v1 "anthonyuk.dev/erspan-hub/generated/streams/v1"
	"anthonyuk.dev/erspan-hub/internal"
	"anthonyuk.dev/erspan-hub/internal/forward"

	"google.golang.org/grpc"
)

type GrpcServer struct {
	logger *slog.Logger
	config *Config
	fsm    *forward.ForwardSessionManager
}

// RunServer will start a gRPC server on the given port
func RunServer(cfg *Config, fsm *forward.ForwardSessionManager) error {
	gsvr := &GrpcServer{
		logger: fsm.Logger(),
		config: cfg,
		fsm:    fsm,
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.BindIP, cfg.Port))
	if err != nil {
		return fmt.Errorf("failed to listen for gRPC: %v", err)
	}

	s := grpc.NewServer()

	streams_v1.RegisterStreamsServiceServer(s, &StreamsServiceServer{gsvr: gsvr})
	pcap_v1.RegisterPcapForwarderServer(s, &PcapForwarderServer{gsvr: gsvr})
	pcap_v1.RegisterValidateFilterServiceServer(s, &ValidateFilterServer{gsvr: gsvr})

	// start server
	go func() {
		gsvr.logger.Info("▶️  gRPC server listening on " + lis.Addr().String())
		if err := s.Serve(lis); err != nil {
			gsvr.logger.Error("gRPC server error", "error", err)
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	gsvr.logger.Info("🛑 Shutting down gRPC server due to signal…")
	fsm.CloseAllForwardSessions(internal.ForwardSessionMsgTypeShutdown)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	<-ctx.Done()
	s.GracefulStop()
	//s.Stop()
	return nil
}
