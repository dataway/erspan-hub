package hubcap

import (
	"context"
	"log/slog"

	pcap_v1 "anthonyuk.dev/erspan-hub/generated/pcap/v1"
	"anthonyuk.dev/erspan-hub/internal/client"
)

func NewClient(cfg *Config, logger *slog.Logger) (*client.Client, context.Context, error) {
	client_cfg := &client.Config{
		GrpcUrl: cfg.GrpcUrl,
	}
	cl, err := client.NewClient(client_cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	ctx := context.Background()
	return cl, ctx, nil
}

func ListStreams(cfg *Config, logger *slog.Logger) (streams []*client.StreamInfo, err error) {
	cl, ctx, err := NewClient(cfg, logger)
	if err != nil {
		return nil, err
	}
	streams, err2 := cl.ListStreams(ctx)
	if err2 != nil {
		return nil, err2
	}
	return streams, nil
}

func ValidateFilter(cfg *Config, logger *slog.Logger) (valid bool, errMsg string, bpf []*pcap_v1.BPFInstruction, err error) {
	cl, ctx, err1 := NewClient(cfg, logger)
	if err1 != nil {
		return false, "", nil, err1
	}
	valid, errMsg, bpf, err = cl.ValidateFilter(ctx, cfg.Filter)
	if err != nil {
		return false, "", nil, err
	}
	if !valid {
		return false, errMsg, nil, nil
	}
	return true, "", bpf, nil
}
