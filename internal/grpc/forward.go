package grpc

import (
	"fmt"
	"time"

	pcap_v1 "anthonyuk.dev/erspan-hub/generated/pcap/v1"
	"anthonyuk.dev/erspan-hub/internal"
	"anthonyuk.dev/erspan-hub/internal/forward"
	"google.golang.org/grpc/peer"
)

type ForwardSessionGrpc struct {
	forward.ForwardSessionBase
	peer *peer.Peer
}

func (fs *ForwardSessionGrpc) GetInfo() map[string]string {
	info := map[string]string{
		"type": "grpc_pcap",
	}
	if fs.peer != nil {
		info["peer_addr"] = fs.peer.Addr.String()
		info["local_addr"] = fs.peer.LocalAddr.String()
	}
	return info
}

func NewForwardSessionGrpc(fsm *forward.ForwardSessionManager, key forward.StreamKey, streamID string, handlerType string, filter string, cfg map[string]any) (fs forward.ForwardSessionChannel, err error) {
	fsm.Logger().Info("gRPC forward session requested", "forward_session_manager", fsm, "config", cfg)

	fsb, err := forward.NewForwardSessionBase(fsm, key, streamID, handlerType, filter, cfg)
	if err != nil {
		return nil, err
	}
	fs_grpc := &ForwardSessionGrpc{
		ForwardSessionBase: *fsb,
	}
	if p, ok := cfg["peer"]; ok {
		if pr, ok := p.(*peer.Peer); ok {
			fs_grpc.peer = pr
		}
	}
	return fs_grpc, nil
}

type PcapForwarderServer struct {
	gsvr *GrpcServer
	pcap_v1.UnimplementedPcapForwarderServer
}

type PcapForwarderWriter struct {
	svr pcap_v1.PcapForwarder_ForwardStreamServer
}

func (w *PcapForwarderWriter) Write(p []byte) (n int, err error) {
	err = w.svr.Send(&pcap_v1.Packet{
		Timestamp: time.Now().UnixNano(),
		RawData:   p,
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *PcapForwarderServer) ForwardStream(req *pcap_v1.ForwardRequest, svr pcap_v1.PcapForwarder_ForwardStreamServer) error {
	ctx := svr.Context()

	srcIP := req.GetSrcIp()
	erspanID := uint16(req.GetErspanId())
	streamInfoID := req.GetStreamInfoId()
	filter := req.GetFilter()

	s.gsvr.logger.InfoContext(ctx, "Received ForwardStream request", "src_ip", srcIP, "erspan_id", erspanID, "stream_info_id", streamInfoID, "filter", filter)
	cfg := make(map[string]any)
	if p, ok := peer.FromContext(ctx); ok {
		cfg["peer"] = p
	}

	fs, err := s.gsvr.fsm.CreateForwardSessionByStreamInfoID(
		streamInfoID,
		"grpc_pcap", filter,
		cfg,
	)
	if err != nil {
		s.gsvr.logger.ErrorContext(ctx, "Failed to create forward session", "error", err)
		return err
	}
	defer s.gsvr.fsm.DeleteForwardSession(fs)
	ch := fs.GetChannel()

	s.gsvr.logger.InfoContext(ctx, "Started gRPC forwarding for stream", "stream_info_id", streamInfoID, "channel", fmt.Sprintf("%+v", ch))
	pfw := &PcapForwarderWriter{svr: svr}
	pcapw, err := forward.NewPcapNgWriter(pfw, fs)
	if err != nil {
		s.gsvr.logger.ErrorContext(ctx, "Failed to create pcapng writer", "error", err)
		return err
	}
	defer pcapw.NgWriter.Flush()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				s.gsvr.logger.InfoContext(ctx, "Forward session channel closed, ending gRPC forwarding", "stream_info_id", streamInfoID)
				return nil
			}
			if msg.Type == internal.ForwardSessionMsgTypePacket {
				if err := pcapw.WritePacket(msg.Packet, msg.Time); err != nil {
					s.gsvr.logger.ErrorContext(ctx, "Failed to write packet via gRPC", "error", err)
				}
			}
		case <-ctx.Done():
			s.gsvr.logger.InfoContext(ctx, "gRPC client context done, ending gRPC forwarding", "stream_info_id", streamInfoID)
			return nil
		}
	}
}

func init() {
	forward.RegisterForwardSessionType("grpc_pcap", NewForwardSessionGrpc)
}
