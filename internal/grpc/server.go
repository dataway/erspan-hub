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

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("failed to listen for gRPC: %v", err)
	}

	s := grpc.NewServer()

	streams_v1.RegisterStreamsServiceServer(s, &StreamsServiceServer{gsvr: gsvr})
	pcap_v1.RegisterPcapForwarderServer(s, &PcapForwarderServer{gsvr: gsvr})

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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	s.GracefulStop()
	defer cancel()
	<-ctx.Done()
	s.Stop()
	return nil
}

type StreamsServiceServer struct {
	gsvr *GrpcServer
	streams_v1.UnimplementedStreamsServiceServer
}

func (s *StreamsServiceServer) ListStreams(ctx context.Context, req *streams_v1.ListStreamsRequest) (*streams_v1.ListStreamsResponse, error) {
	s.gsvr.fsm.RLock()
	defer s.gsvr.fsm.RUnlock()

	resp := &streams_v1.ListStreamsResponse{}
	for id, info := range s.gsvr.fsm.Streams {
		sinfo := streams_v1.StreamInfo{
			Id:              info.ID,
			SrcIp:           uint32(id.SrcIP.ToUint32()),
			ErspanId:        uint32(id.ErspanID),
			ErspanVersion:   uint32(info.ErspanVersion),
			FirstSeen:       info.FirstSeen.UnixNano(),
			LastSeen:        info.LastSeen.UnixNano(),
			Packets:         info.Packets,
			Bytes:           info.Bytes,
			ForwardSessions: make([]*streams_v1.ForwardSession, 0, len(info.ForwardSessions)),
		}
		for fs := range info.ForwardSessions {
			sinfo_fs := streams_v1.ForwardSession{
				SrcIp:        uint32(fs.GetStreamKey().SrcIP.ToUint32()),
				ErspanId:     uint32(fs.GetStreamKey().ErspanID),
				StreamInfoId: fs.GetStreamInfoID(),
				Type:         fs.GetType(),
				Filter:       fs.GetFilterString(),
				Info:         fs.GetInfo(),
			}
			sinfo.ForwardSessions = append(sinfo.ForwardSessions, &sinfo_fs)
		}
		resp.Streams = append(resp.Streams, &sinfo)
	}
	return resp, nil
}
