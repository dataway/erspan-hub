package forward

import (
	"fmt"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

func NewForwardSessionBase(fsm *ForwardSessionManager, key StreamKey, streamID string, handlerType string, filter string, cfg map[string]any) (fsb *ForwardSessionBase, err error) {
	ch := make(chan ForwardSessionMsg, 32)
	sess := &ForwardSessionBase{
		StreamKey:    key,
		StreamInfoID: streamID,
		Type:         handlerType,
		Channel:      ch,
	}
	if filter != "" {
		bpfFilter, err := pcap.NewBPF(layers.LinkTypeEthernet, 65535, filter)
		if err != nil {
			return nil, fmt.Errorf("bad filter: %v", err)
		}
		sess.Filter = bpfFilter
	}
	return sess, nil
}

func NewForwardSessionBaseFactory(fsm *ForwardSessionManager, key StreamKey, streamID string, handlerType string, filter string, cfg map[string]any) (fs ForwardSessionChannel, err error) {
	return NewForwardSessionBase(fsm, key, streamID, handlerType, filter, cfg)
}

// CreateForwardSessionByStreamInfoID creates a new ForwardSession for the given StreamInfo ID
func (fsm *ForwardSessionManager) CreateForwardSessionByStreamInfoID(streamInfoID string, handlerType string, filter string, cfg map[string]any) (ForwardSessionChannel, error) {
	_, key := fsm.GetStreamByID(streamInfoID)
	if key == NullStreamKey {
		return nil, fmt.Errorf("stream not found: %s", streamInfoID)
	}
	return fsm.createForwardSessionImpl(key, streamInfoID, handlerType, filter, cfg)
}

// CreateForwardSessionByKey creates a new ForwardSession for the given StreamKey
func (fsm *ForwardSessionManager) CreateForwardSessionByKey(key StreamKey, handlerType string, filter string, cfg map[string]any) (ForwardSessionChannel, error) {
	return fsm.createForwardSessionImpl(key, "", handlerType, filter, cfg)
}

// Common code used by both CreateForwardSessionByStreamInfoID and CreateForwardSessionByKey
// to actually create and register the ForwardSession
func (fsm *ForwardSessionManager) createForwardSessionImpl(key StreamKey, streamInfoID string, handlerType string, filter string, cfg map[string]any) (ForwardSessionChannel, error) {
	factory, ok := ForwardSessionTypes[handlerType]
	if !ok {
		return nil, fmt.Errorf("unknown forward session type: %s", handlerType)
	}
	if factory == nil {
		panic("factory function is nil for registered forward session type: " + handlerType)
	}
	fs, err := factory(fsm, key, streamInfoID, handlerType, filter, cfg)
	if err != nil {
		fsm.logger.Error("Failed to create forward session", "error", err)
		return nil, err
	}

	si, exists := fsm.GetStream(fs.GetStreamKey())
	if !exists {
		// TODO: pending stream functionality
		return nil, fmt.Errorf("stream not found: %s", fs.GetStreamKey())
	}
	fsm.Lock()
	defer fsm.Unlock()
	si.ForwardSessions[fs] = struct{}{}
	fsm.Streams[fs.GetStreamKey()] = si
	fsm.logger.Info("Created new forward session", "stream_id", si.ID, "type", handlerType, "fs", fs)
	return fs, nil
}

// DeleteForwardSession removes a ForwardSession from the manager and cleans up
func (fsm *ForwardSessionManager) DeleteForwardSession(fs ForwardSessionChannel) {
	si, exists := fsm.GetStream(fs.GetStreamKey())
	fsm.Lock()
	if exists {
		delete(si.ForwardSessions, fs)
	}
	// Remove from the global session set
	delete(fsm.Sessions, fs)
	fsm.Unlock()
	// Close the channel to signal the receiver to stop
	close(fs.GetChannel())
	fsm.logger.Info("Deleted forward session", "stream_id", si.ID, "fs", fs)
}
