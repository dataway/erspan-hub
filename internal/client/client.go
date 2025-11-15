package client

import (
	"context"
	"log/slog"
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
	Config  *Config
	Logger  *slog.Logger
	conn    *grpc.ClientConn
	streams streams_v1.StreamsServiceClient
	pcap    pcap_v1.PcapForwarderClient
}

func NewClient(cfg *Config, logger *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(cfg.GrpcUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("did not connect", "error", err)
		return nil, err
	}
	client := &Client{
		Config:  cfg,
		Logger:  logger,
		conn:    conn,
		streams: streams_v1.NewStreamsServiceClient(conn),
		pcap:    pcap_v1.NewPcapForwarderClient(conn),
	}
	return client, nil
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = nil
}

func (c *Client) ListStreams(ctx context.Context) (streams []*StreamInfo, err error) {
	resp, err := c.streams.ListStreams(ctx, &streams_v1.ListStreamsRequest{})
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
