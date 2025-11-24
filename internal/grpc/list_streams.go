package grpc

import (
	"context"

	streams_v1 "anthonyuk.dev/erspan-hub/generated/streams/v1"
)

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
