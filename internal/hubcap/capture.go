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

func RunCapture(cfg *Config, logger *slog.Logger, clientInfo map[string]string) (err error) {
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

	var ctrlOut *os.File = nil
	var ctrlCh chan ExtcapControlPkt
	if cfg.ExtcapControlIn != "" {
		ctrlCh = ExtcapControlReceiver(cfg, logger)
		if ctrlCh != nil {
			go func() {
				for pkt := range ctrlCh {
					logger.Info("Extcap control packet received", "ctrl", pkt.Ctrl, "cmd", pkt.Cmd, "payload", string(pkt.Payload))
				}
			}()
		}
	}
	if cfg.ExtcapControlOut != "" {
		ctrlOut, err = os.OpenFile(cfg.ExtcapControlOut, os.O_WRONLY, 0)
		if err != nil {
			logger.Error("failed to open extcap control out", "file", cfg.ExtcapControlOut, "error", err)
			return fmt.Errorf("failed to open extcap control out: %w", err)
		}
		defer ctrlOut.Close()
	}

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

	stream, err := cl.PcapClient.ForwardStream(ctx, &pcap_v1.ForwardRequest{StreamInfoId: streamID, Filter: cfg.Filter, ClientInfo: clientInfo})
	if err != nil {
		logger.Error("could not subscribe to stream", "error", err)
		return err
	}

	count := uint64(0)
	blockCount := uint64(0)
	cts := time.Now()
	testMode := cfg.TestCapture
	for {
		packet, err := stream.Recv()
		if err != nil {
			if ctrlOut != nil {
				ExtcapControlSend(ctrlOut, ExtcapControlPkt{
					Ctrl: 1, Cmd: 8, Payload: []byte(fmt.Sprintf("Error receiving packet from ERSPAN hub server: %v", err)),
				})
			}
			logger.Error("error receiving packet", "error", err)
			return err
		}
		if packet.Timestamp < 0 {
			if ctrlOut != nil {
				ExtcapControlSend(ctrlOut, ExtcapControlPkt{
					Ctrl: 1, Cmd: 7, Payload: []byte("Capture ended by ERSPAN hub server"),
				})
			}
			logger.Info("Server closed capture stream")
			return nil
		}
		blockCount++
		count += uint64(packet.PacketCount)
		fifo.Write(packet.RawData)
		if testMode {
			now := time.Now()
			if now.Sub(cts) >= time.Second {
				fmt.Printf("\rReceived %10d packets in %10d blocks", count, blockCount)
				cts = now
			}
		}
	}
}
