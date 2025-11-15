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

// MarshalJSON implements custom JSON marshalling for ForwardSession to include the filter string
func (fs *ForwardSessionBase) MarshalJSON() ([]byte, error) {
	type Alias ForwardSessionBase
	var filterStr string
	if fs.Filter != nil {
		filterStr = fs.Filter.String()
	}
	return json.Marshal(&struct {
		Filter string `json:"filter"`
		*Alias
	}{
		Filter: filterStr,
		Alias:  (*Alias)(fs),
	})
}
