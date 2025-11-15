package forward

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"anthonyuk.dev/erspan-hub/internal"
)

type ForwardSessionUDP struct {
	ForwardSessionBase
}

func NewForwardSessionUDP(fsm *ForwardSessionManager, key StreamKey, streamID string, handlerType string, filter string, cfg map[string]any) (fs ForwardSessionChannel, err error) {
	fsm.logger.Info("UDP forward session requested", "forward_session_manager", fsm, "config", cfg)

	fsb, err := NewForwardSessionBase(fsm, key, streamID, handlerType, filter, cfg)
	if err != nil {
		return nil, err
	}
	fs_udp := &ForwardSessionUDP{
		ForwardSessionBase: *fsb,
	}
	ch := fsb.Channel

	destIP := net.ParseIP(string(cfg["dest_ip"].(string)))
	destPort := uint16(cfg["dest_port"].(float64))
	if destIP.To4() == nil {
		return nil, fmt.Errorf("only IPv4 is supported for UDP forwarding")
	}
	addr := &net.UDPAddr{
		IP:   destIP,
		Port: int(destPort),
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP connection to %s: %w", addr.String(), err)
	}

	logEconnrefused := true
	go func() {
		for msg := range ch {
			if msg.Type == internal.ForwardSessionMsgTypePacket {
				if _, err := conn.Write(msg.Packet); err != nil {
					if errors.Is(err, syscall.ECONNREFUSED) {
						// Only report ECONNREFUSED once
						if logEconnrefused {
							fsm.logger.Warn("ECONNREFUSED error forwarding UDP packet (will not warn again)", "forward_session", fs, "error", err)
						}
						logEconnrefused = false
					} else {
						fsm.logger.Error("Error forwarding UDP packet", "forward_session", fs, "error", err)
					}
				}
			}
			if msg.Type == internal.ForwardSessionMsgTypeClose {
				break
			}
		}
		conn.Close()
	}()
	return fs_udp, nil
}

func init() {
	RegisterForwardSessionType("udp", NewForwardSessionUDP)
}
