package forward

import (
	"sync"
	"time"

	"anthonyuk.dev/erspan-hub/internal"
	"github.com/google/gopacket"
)

type ForwardSessionMsg = internal.ForwardSessionMsg
type ForwardSessionMsgType = internal.ForwardSessionMsgType

func (fsm *ForwardSessionManager) ProcessPacket(key StreamKey, timestamp time.Time, packet []byte) {
	// Register or update discovered stream
	var si = fsm.UpdateStream(key, timestamp, len(packet))

	// Forward to matching sessions
	fsm.ForwardToSessions(si, timestamp, packet)
}

// ForwardToSessions forwards a packet to all matching forwarding sessions
func (fsm *ForwardSessionManager) ForwardToSessions(si *StreamInfo, timestamp time.Time, payload []byte) {
	// Copy the array of relevant channels (which pass the BPF filter) to avoid holding the lock while sending
	fsm.RLock()
	channels := make([]chan ForwardSessionMsg, 0, len(si.ForwardSessions))
	gci := gopacket.CaptureInfo{Timestamp: timestamp, CaptureLength: len(payload), Length: len(payload)}
	for sess := range si.ForwardSessions {
		schan := sess.(ForwardSessionChannel)
		if schan.GetBpfFilter() != nil && !schan.GetBpfFilter().Matches(gci, payload) {
			continue
		}
		channels = append(channels, schan.GetChannel())
	}
	fsm.RUnlock()

	msg := ForwardSessionMsg{
		Type:   internal.ForwardSessionMsgTypePacket,
		Packet: payload,
		Time:   timestamp,
	}
	wg := sync.WaitGroup{}
	for _, ch := range channels {
		wg.Add(1)
		go func(ch chan ForwardSessionMsg) {
			defer wg.Done()
			select {
			case ch <- msg:
			case <-time.After(100 * time.Millisecond):
				fsm.logger.Warn("Dropping packet for slow forward session")
			}
		}(ch)
	}
	wg.Wait()
}
