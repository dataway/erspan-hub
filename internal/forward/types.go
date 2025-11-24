package forward

import (
	"encoding/json"
	"sync/atomic"

	"anthonyuk.dev/erspan-hub/internal"

	"github.com/google/gopacket/pcap"
)

type StreamKey = internal.StreamKey
type StreamInfo = internal.StreamInfo
type ForwardSessionSet = internal.ForwardSessionSet

type ForwardSessionStats struct {
	StartTime       int64         `json:"start_time"`
	TotalPackets    atomic.Uint64 `json:"total_packets"`
	FilteredPackets atomic.Uint64 `json:"filtered_packets"`
	// number of packets in the session is TotalPackets - FilteredPackets
}

type ForwardSessionBase struct { // implements ForwardSession
	StreamKey    StreamKey              `json:"stream_key"`
	StreamInfoID string                 `json:"stream_info_id"`
	Type         string                 `json:"type"`
	Filter       *pcap.BPF              `json:"-"`
	Channel      chan ForwardSessionMsg `json:"-"`
	Stats        *ForwardSessionStats   `json:"stats"`
}

type ForwardSessionChannel interface {
	GetBpfFilter() *pcap.BPF
	GetChannel() chan ForwardSessionMsg
	GetStats() *ForwardSessionStats
	internal.ForwardSession
}

func (fs *ForwardSessionBase) GetBpfFilter() *pcap.BPF {
	return fs.Filter
}

func (fs *ForwardSessionBase) GetChannel() chan ForwardSessionMsg {
	return fs.Channel
}

func (fs *ForwardSessionBase) GetStats() *ForwardSessionStats {
	return fs.Stats
}

func (fs *ForwardSessionBase) GetStatsMap() *map[string]any {
	return &map[string]any{
		"start_time":       fs.Stats.StartTime,
		"total_packets":    fs.Stats.TotalPackets.Load(),
		"filtered_packets": fs.Stats.FilteredPackets.Load(),
	}
}

func (fs *ForwardSessionBase) GetStreamKey() StreamKey {
	return fs.StreamKey
}

func (fs *ForwardSessionBase) GetStreamInfoID() string {
	return fs.StreamInfoID
}

func (fs *ForwardSessionBase) GetType() string {
	return fs.Type
}

func (fs *ForwardSessionBase) GetFilterString() string {
	if fs.Filter != nil {
		return fs.Filter.String()
	}
	return ""
}

func (fs *ForwardSessionBase) GetInfo() map[string]string {
	return map[string]string{}
}

// forwardSessionInfo is used for JSON marshalling of ForwardSession

type forwardSessionInfo struct {
	StreamKey    StreamKey         `json:"stream_key"`
	StreamInfoID string            `json:"stream_info_id"`
	Type         string            `json:"type"`
	Filter       string            `json:"filter"`
	Info         map[string]string `json:"info,omitempty"`
	Stats        *map[string]any   `json:"stats,omitempty"`
}

func MarshalJSONIntf(fs internal.ForwardSession) ([]byte, error) {
	return json.Marshal(forwardSessionInfo{
		StreamKey:    fs.GetStreamKey(),
		StreamInfoID: fs.GetStreamInfoID(),
		Type:         fs.GetType(),
		Filter:       fs.GetFilterString(),
		Info:         fs.GetInfo(),
		Stats:        fs.GetStatsMap(),
	})
}

// This method is needed to satisfy the interface and must be implemented by all interfaces
// derived from ForwardSessionBase to ensure overrides are correctly called.
func (fs *ForwardSessionBase) MarshalJSON() ([]byte, error) {
	return MarshalJSONIntf(fs)
}
