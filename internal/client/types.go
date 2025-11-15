package client

import (
	"net"
	"time"
)

type StreamInfo struct {
	ID              string                `json:"id"`
	SrcIP           net.IP                `json:"src_ip"`
	ErspanID        uint16                `json:"erspan_id"`
	ErspanVersion   uint8                 `json:"erspan_version"`
	FirstSeen       time.Time             `json:"first_seen"`
	LastSeen        time.Time             `json:"last_seen"`
	Packets         uint64                `json:"packets"`
	Bytes           uint64                `json:"bytes"`
	ForwardSessions []*ForwardSessionInfo `json:"forward_sessions"`
}

type ForwardSessionInfo struct {
	SrcIP        net.IP            `json:"src_ip"`
	ErspanID     uint16            `json:"erspan_id"`
	StreamInfoID string            `json:"stream_info_id"`
	Type         string            `json:"type"`
	Filter       string            `json:"filter"`
	Info         map[string]string `json:"info"`
}

func IPFromUint32(ip uint32) net.IP {
	return net.IPv4(byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip))
}
