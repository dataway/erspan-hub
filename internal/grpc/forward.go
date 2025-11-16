package grpc

import (
	"context"
	"fmt"
	"time"

	pcap_v1 "anthonyuk.dev/erspan-hub/generated/pcap/v1"
	"anthonyuk.dev/erspan-hub/internal"
	"anthonyuk.dev/erspan-hub/internal/forward"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
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
	fmt.Printf("ForwardSessionGrpc GetInfo: %+v\n", info)
	return info
}

func (fs *ForwardSessionGrpc) MarshalJSON() ([]byte, error) {
	return forward.MarshalJSONIntf(fs)
}

func NewForwardSessionGrpc(fsm *forward.ForwardSessionManager, key forward.StreamKey, streamID string, handlerType string, filter string, cfg map[string]any) (fs forward.ForwardSessionChannel, err error) {
	fsm.Logger().Info("gRPC forward session requested", "config", cfg)

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

	s.gsvr.logger.DebugContext(ctx, "Received ForwardStream request", "src_ip", srcIP, "erspan_id", erspanID, "stream_info_id", streamInfoID, "filter", filter)
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
				s.gsvr.logger.DebugContext(ctx, "Forward session channel closed, ending gRPC forwarding", "stream_info_id", streamInfoID)
				return nil
			}
			switch msg.Type {
			case internal.ForwardSessionMsgTypePacket:
				if err := pcapw.WritePacket(msg.Packet, msg.Time); err != nil {
					s.gsvr.logger.ErrorContext(ctx, "Failed to write packet via gRPC", "error", err)
				}

			case internal.ForwardSessionMsgTypeClose:
				pcapw.NgWriter.Flush()
				svr.Send(&pcap_v1.Packet{
					Timestamp: -1,
					RawData:   nil,
				})
				return nil

			case internal.ForwardSessionMsgTypeShutdown:
				pcapw.NgWriter.Flush()
				svr.Send(&pcap_v1.Packet{
					Timestamp: -2,
					RawData:   nil,
				})
				return nil
			}
		case <-ctx.Done():
			s.gsvr.logger.DebugContext(ctx, "gRPC client context done, ending gRPC forwarding", "stream_info_id", streamInfoID)
			return nil
		}
	}
}

type ValidateFilterServer struct {
	gsvr *GrpcServer
	pcap_v1.UnimplementedValidateFilterServiceServer
}

func (s *ValidateFilterServer) ValidateFilter(ctx context.Context, req *pcap_v1.ValidateFilterRequest) (*pcap_v1.ValidateFilterResponse, error) {
	filter := req.GetFilter()
	s.gsvr.logger.DebugContext(ctx, "Received ValidateFilter request", "filter", filter)

	resp := &pcap_v1.ValidateFilterResponse{}

	bpf, err := pcap.CompileBPFFilter(layers.LinkTypeEthernet, 65535, filter)
	if err == nil {
		resp.Valid = true
		resp.Bpf = make([]*pcap_v1.BPFInstruction, len(bpf))
		for i, ins := range bpf {
			resp.Bpf[i] = &pcap_v1.BPFInstruction{
				Code: uint32(ins.Code),
				Jt:   uint32(ins.Jt),
				Jf:   uint32(ins.Jf),
				K:    ins.K,
			}
		}
	} else {
		resp.Valid = false
		resp.ErrorMessage = fmt.Sprintf("%v", err)
	}
	return resp, nil
}

func init() {
	forward.RegisterForwardSessionType("grpc_pcap", NewForwardSessionGrpc)
}
