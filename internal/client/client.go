package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"os"
	"time"

	pcap_v1 "anthonyuk.dev/erspan-hub/generated/pcap/v1"
	streams_v1 "anthonyuk.dev/erspan-hub/generated/streams/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	GrpcUrl         string
	GrpcTLS         bool
	GrpcTLSInsecure bool
	GrpcTLSCAFile   string
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
	var opts []grpc.DialOption
	if cfg.GrpcTLS {
		// Set up options for TLS connection
		tlsConfig := &tls.Config{
			ServerName: cfg.GrpcUrl,
		}
		if cfg.GrpcTLSInsecure {
			tlsConfig.InsecureSkipVerify = true
		} else if cfg.GrpcTLSCAFile != "" {
			caCert, err := os.ReadFile(cfg.GrpcTLSCAFile)
			if err != nil {
				logger.Error("failed to read CA file", "path", cfg.GrpcTLSCAFile, "error", err)
				return nil, err
			}
			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				logger.Error("CA file did not contain a valid PEM certificate", "path", cfg.GrpcTLSCAFile)
				return nil, errors.New("invalid CA certificate file")
			}
			tlsConfig.RootCAs = certPool
			logger.Info("Using custom CA file for server certificate verification.", "path", cfg.GrpcTLSCAFile)
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		// Non-TLS connection
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	logger.Info("Connecting to gRPC server", "url", cfg.GrpcUrl, "opts", opts)
	conn, err := grpc.NewClient(cfg.GrpcUrl, opts...)
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
		return false, "", nil, err
	}
	return resp.Valid, resp.ErrorMessage, resp.Bpf, nil
}
