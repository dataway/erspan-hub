package forward

import (
	"encoding/json"

	"anthonyuk.dev/erspan-hub/internal"
	"github.com/google/gopacket/pcap"
)

type StreamKey = internal.StreamKey
type StreamInfo = internal.StreamInfo
type ForwardSessionSet = internal.ForwardSessionSet

type ForwardSessionBase struct { // implements ForwardSession
	StreamKey    StreamKey              `json:"stream_key"`
	StreamInfoID string                 `json:"stream_info_id"`
	Type         string                 `json:"type"`
	Filter       *pcap.BPF              `json:"-"`
	Channel      chan ForwardSessionMsg `json:"-"`
}

type ForwardSessionChannel interface {
	GetBpfFilter() *pcap.BPF
	GetChannel() chan ForwardSessionMsg
	internal.ForwardSession
}

func (fs *ForwardSessionBase) GetBpfFilter() *pcap.BPF {
	return fs.Filter
}

func (fs *ForwardSessionBase) GetChannel() chan ForwardSessionMsg {
	return fs.Channel
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
}

func MarshalJSONIntf(fs internal.ForwardSession) ([]byte, error) {
	return json.Marshal(forwardSessionInfo{
		StreamKey:    fs.GetStreamKey(),
		StreamInfoID: fs.GetStreamInfoID(),
		Type:         fs.GetType(),
		Filter:       fs.GetFilterString(),
		Info:         fs.GetInfo(),
	})
}

// This method is needed to satisfy the interface and must be implemented by all interfaces
// derived from ForwardSessionBase to ensure overrides are correctly called.
func (fs *ForwardSessionBase) MarshalJSON() ([]byte, error) {
	return MarshalJSONIntf(fs)
}
