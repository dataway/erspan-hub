package internal

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// IPv4 represents a 4-byte IPv4 address that can be used as a map key
type IPv4 [4]byte

func (ip IPv4) ToNetIP() net.IP {
	return net.IP(ip[:])
}

func (ip IPv4) String() string {
	return net.IP(ip[:]).String()
}

func (ip IPv4) ToUint32() uint32 {
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func IPFromUint32(u uint32) IPv4 {
	return IPv4{
		byte((u >> 24) & 0xFF),
		byte((u >> 16) & 0xFF),
		byte((u >> 8) & 0xFF),
		byte(u & 0xFF),
	}
}

func IPv4FromString(s string) IPv4 {
	var ip IPv4
	parsed := net.ParseIP(s).To4()
	if parsed != nil {
		copy(ip[:], parsed)
	}
	return ip
}

// StreamKey uniquely identifies an ERSPAN stream by source IP and ERSPAN session ID
type StreamKey struct {
	SrcIP    IPv4   `json:"src_ip"`
	ErspanID uint16 `json:"erspan_id"`
}

var NullStreamKey = StreamKey{SrcIP: IPv4{0, 0, 0, 0}, ErspanID: 65535}

func (sk StreamKey) String() string {
	return fmt.Sprintf("%s/%d", sk.SrcIP.String(), sk.ErspanID)
}

type StreamInfo struct {
	ID              string            `json:"id"`
	SrcIP           IPv4              `json:"src_ip"`
	ErspanID        uint16            `json:"erspan_id"`
	ErspanVersion   uint8             `json:"erspan_version"`
	FirstSeen       time.Time         `json:"first_seen"`
	LastSeen        time.Time         `json:"last_seen"`
	Packets         uint64            `json:"packets"`
	Bytes           uint64            `json:"bytes"`
	ForwardSessions ForwardSessionSet `json:"forward_sessions"`
}

// ForwardSession represents a session forwarding packets from a specific ERSPAN stream
// Multiple forward sessions can be created for the same stream

type ForwardSession interface {
	GetStreamKey() StreamKey
	GetStreamInfoID() string
	GetType() string
	GetFilterString() string
	GetInfo() map[string]string
	MarshalJSON() ([]byte, error)
}

type ForwardSessionSet map[ForwardSession]struct{}

type ForwardSessionMsgType uint

const (
	ForwardSessionMsgTypePacket ForwardSessionMsgType = iota
	ForwardSessionMsgTypeClose
)

type ForwardSessionMsg struct {
	Type   ForwardSessionMsgType
	Packet []byte
	Time   time.Time
}

// MarshalJSON implements custom JSON marshalling for ForwardSessionSet
func (s ForwardSessionSet) MarshalJSON() ([]byte, error) {
	if s == nil {
		return json.Marshal(nil)
	}
	sessionList := make([]ForwardSession, 0, len(s))
	for sessionPtr := range s {
		sessionList = append(sessionList, sessionPtr)
	}
	return json.Marshal(sessionList)
}
