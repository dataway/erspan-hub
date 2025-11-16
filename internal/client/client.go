package client

import (
	"context"
	"log/slog"
	"os"
	"time"

	pcap_v1 "anthonyuk.dev/erspan-hub/generated/pcap/v1"
	streams_v1 "anthonyuk.dev/erspan-hub/generated/streams/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	GrpcUrl string
}

type Client struct {
	Config               *Config
	Logger               *slog.Logger
	Conn                 *grpc.ClientConn
	StreamsClient        streams_v1.StreamsServiceClient
	PcapClient           pcap_v1.PcapForwarderClient
	ValidateFilterClient pcap_v1.ValidateFilterServiceClient
}

func NewClient(cfg *Config, logger *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(cfg.GrpcUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("did not connect", "error", err)
		return nil, err
	}
	client := &Client{
		Config:               cfg,
		Logger:               logger,
		Conn:                 conn,
		StreamsClient:        streams_v1.NewStreamsServiceClient(conn),
		PcapClient:           pcap_v1.NewPcapForwarderClient(conn),
		ValidateFilterClient: pcap_v1.NewValidateFilterServiceClient(conn),
	}
	return client, nil
}

func NewClientOrExit(cfg *Config, logger *slog.Logger) *Client {
	client, err := NewClient(cfg, logger)
	if err != nil {
		logger.Error("failed to create client", "error", err)
		os.Exit(1)
	}
	return client
}

func (c *Client) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
	c.Conn = nil
}

func (c *Client) ListStreams(ctx context.Context) (streams []*StreamInfo, err error) {
	resp, err := c.StreamsClient.ListStreams(ctx, &streams_v1.ListStreamsRequest{})
	if err != nil {
		c.Logger.Error("could not list streams", "error", err)
		return nil, err
	}
	c.Logger.Debug("ListStreams", "streams", resp.Streams)
	streams = make([]*StreamInfo, 0, len(resp.Streams))
	for _, stream := range resp.Streams {
		sinfo := StreamInfo{
			ID:              stream.Id,
			SrcIP:           IPFromUint32(stream.SrcIp),
			ErspanID:        uint16(stream.ErspanId),
			ErspanVersion:   uint8(stream.ErspanVersion),
			FirstSeen:       time.Unix(0, stream.FirstSeen),
			LastSeen:        time.Unix(0, stream.LastSeen),
			Packets:         stream.Packets,
			Bytes:           stream.Bytes,
			ForwardSessions: make([]*ForwardSessionInfo, 0, len(stream.ForwardSessions)),
		}
		for _, session := range stream.ForwardSessions {
			sinfo.ForwardSessions = append(sinfo.ForwardSessions, &ForwardSessionInfo{
				SrcIP:        IPFromUint32(session.SrcIp),
				ErspanID:     uint16(session.ErspanId),
				StreamInfoID: sinfo.ID,
				Type:         session.Type,
				Filter:       session.Filter,
				Info:         session.Info,
			})
		}
		streams = append(streams, &sinfo)
	}
	return streams, nil
}

func (c *Client) ValidateFilter(ctx context.Context, filter string) (valid bool, errMsg string, bpf []*pcap_v1.BPFInstruction, err error) {
	var resp *pcap_v1.ValidateFilterResponse
	resp, err = c.ValidateFilterClient.ValidateFilter(ctx, &pcap_v1.ValidateFilterRequest{
		Filter: filter,
	})
	if err != nil {
		c.Logger.Error("could not validate filter", "error", err)
		return false, "", nil, err
	}
	return resp.Valid, resp.ErrorMessage, resp.Bpf, nil
}
