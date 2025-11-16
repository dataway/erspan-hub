package hubcap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	pcap_v1 "anthonyuk.dev/erspan-hub/generated/pcap/v1"
	streams_v1 "anthonyuk.dev/erspan-hub/generated/streams/v1"
	"anthonyuk.dev/erspan-hub/internal/client"
)

func RunCapture(cfg *Config, logger *slog.Logger) (err error) {
	fifo, err := os.OpenFile(cfg.Fifo, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		logger.Error("Failed to open fifo for writing", "fifo", cfg.Fifo, "error", err)
		return err
	}
	defer fifo.Close()

	cl := client.NewClientOrExit(&client.Config{GrpcUrl: cfg.GrpcUrl}, logger)
	defer cl.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamID := cfg.StreamID
	if cfg.StreamID == "" {
		resp, err := cl.StreamsClient.ListStreams(ctx, &streams_v1.ListStreamsRequest{})
		if err != nil {
			logger.Error("could not list streams", "error", err)
			return err
		}
		logger.Debug("Streams", "streams", resp.Streams)

		if len(resp.Streams) == 0 {
			logger.Info("No streams available")
			return nil
		}
		streamID = resp.Streams[0].Id
	}
	logger.DebugContext(ctx, "Start capturing", "streamID", streamID, "fifo", cfg.Fifo, "filter", cfg.Filter)

	stream, err := cl.PcapClient.ForwardStream(ctx, &pcap_v1.ForwardRequest{StreamInfoId: streamID, Filter: cfg.Filter})
	if err != nil {
		logger.Error("could not subscribe to stream", "error", err)
		return err
	}

	c := 0
	cts := time.Now()
	testMode := cfg.TestCapture
	for {
		packet, err := stream.Recv()
		if err != nil {
			logger.Error("error receiving packet", "error", err)
			return err
		}
		if packet.Timestamp < 0 {
			logger.Warn("Received packet with negative timestamp", "timestamp", packet.Timestamp)
			return nil
		}
		c++
		fifo.Write(packet.RawData)
		if testMode {
			now := time.Now()
			if now.Sub(cts) >= time.Second {
				fmt.Printf("\rReceived %d packets", c)
				cts = now
			}
		}
	}
}
