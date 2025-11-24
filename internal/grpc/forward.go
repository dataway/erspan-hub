package grpc

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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
	peer       *peer.Peer
	clientInfo *map[string]string
}

func (fs *ForwardSessionGrpc) GetInfo() map[string]string {
	info := map[string]string{
		"type": "grpc_pcap",
	}
	if fs.peer != nil {
		info["peer_addr"] = fs.peer.Addr.String()
		info["local_addr"] = fs.peer.LocalAddr.String()
	}
	if fs.clientInfo != nil {
		for k, v := range *fs.clientInfo {
			info[k] = v
		}
	}
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
	if ci, ok := cfg["client_info"]; ok {
		if cinfo, ok := ci.(map[string]string); ok {
			fs_grpc.clientInfo = &cinfo
		}
	}
	return fs_grpc, nil
}

type PcapForwarderServer struct {
	gsvr *GrpcServer
	pcap_v1.UnimplementedPcapForwarderServer
}

type PcapForwarderWriter struct {
	svr   pcap_v1.PcapForwarder_ForwardStreamServer
	count atomic.Uint32
}

func (w *PcapForwarderWriter) Write(p []byte) (n int, err error) {
	err = w.svr.Send(&pcap_v1.PacketBlock{
		Timestamp:   time.Now().UnixNano(),
		PacketCount: w.count.Swap(0),
		RawData:     p,
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *PcapForwarderServer) ForwardStream(req *pcap_v1.ForwardRequest, svr pcap_v1.PcapForwarder_ForwardStreamServer) error {
	ctx := svr.Context()
	mu := &sync.Mutex{}

	streamInfoID := req.GetStreamInfoId()
	filter := req.GetFilter()

	//Too much logging even for debug...
	//s.gsvr.logger.DebugContext(ctx, "Received ForwardStream request", "src_ip", req.GetSrcIp(), "erspan_id", req.GetErspanId(), "stream_info_id", streamInfoID, "filter", filter)
	cfg := make(map[string]any)
	cfg["client_info"] = req.GetClientInfo()
	if p, ok := peer.FromContext(ctx); ok {
		cfg["peer"] = p
	}

	fs_, err := s.gsvr.fsm.CreateForwardSessionByStreamInfoID(
		streamInfoID,
		"grpc_pcap", filter,
		cfg,
	)
	if err != nil {
		s.gsvr.logger.ErrorContext(ctx, "Failed to create forward session", "error", err)
		return err
	}
	fs := fs_.(*ForwardSessionGrpc)
	defer s.gsvr.fsm.DeleteForwardSession(fs)
	ch := fs.GetChannel()

	pfw := &PcapForwarderWriter{svr: svr}
	pcapw, err := forward.NewPcapNgWriter(pfw, fs)
	if err != nil {
		s.gsvr.logger.ErrorContext(ctx, "Failed to create pcapng writer", "error", err)
		return err
	}
	defer pcapw.NgWriter.Flush()

	// Run a goroutine to flush pcapng writer periodically
	ticker := time.NewTicker(200 * time.Millisecond)
	go func() {
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				pcapw.NgWriter.Flush()
				mu.Unlock()
			case <-ctx.Done():
				ticker.Stop()
				mu.Lock()
				pcapw.NgWriter.Flush()
				mu.Unlock()
				return
			}
		}
	}()

	// Main loop to forward packets from channel to gRPC stream
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				s.gsvr.logger.DebugContext(ctx, "Forward session channel closed, ending gRPC forwarding", "stream_info_id", streamInfoID)
				return nil
			}
			switch msg.Type {
			case internal.ForwardSessionMsgTypePacket:
				mu.Lock()
				if err := pcapw.WritePacket(msg.Packet, msg.Time); err != nil {
					s.gsvr.logger.ErrorContext(ctx, "Failed to write packet via gRPC", "error", err)
					mu.Unlock()
					return err
				}
				pfw.count.Add(1)
				mu.Unlock()

			case internal.ForwardSessionMsgTypeClose:
				pcapw.NgWriter.Flush()
				svr.Send(&pcap_v1.PacketBlock{
					Timestamp: -1,
					RawData:   nil,
				})
				return nil

			case internal.ForwardSessionMsgTypeShutdown:
				pcapw.NgWriter.Flush()
				svr.Send(&pcap_v1.PacketBlock{
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
