package hubcap

import (
	"fmt"
	"log/slog"
)

func HandleListStreams(cfg *Config, logger *slog.Logger) (err error) {
	streams, err := ListStreams(cfg, logger)
	if err != nil {
		logger.Error("Failed to list streams", "error", err)
		return err
	}
	if len(streams) == 0 {
		logger.Info("No streams available")
		return nil
	}
	fmt.Printf("Available streams:\n")
	for _, stream := range streams {
		fmt.Printf("ID: %s, SrcIP: %s, ERSPAN ID: %d, Version: %d, FirstSeen: %s, LastSeen: %s, Packets: %d, Bytes: %d\n",
			stream.ID, stream.SrcIP, stream.ErspanID, stream.ErspanVersion, stream.FirstSeen, stream.LastSeen, stream.Packets, stream.Bytes)
		if len(stream.ForwardSessions) > 0 {
			fmt.Printf("  Forward Sessions:\n")
			for _, sess := range stream.ForwardSessions {
				fmt.Printf("    SrcIP: %s, ERSPAN ID: %d, StreamInfoID: %s, Type: %s, Filter: %s, Info: %v\n",
					sess.SrcIP, sess.ErspanID, sess.StreamInfoID, sess.Type, sess.Filter, sess.Info)

			}
		}
	}
	return nil
}
