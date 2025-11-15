package forward

import (
	"crypto/rand"
	"log/slog"
	"sync"
	"time"

	"anthonyuk.dev/erspan-hub/internal"
)

type ForwardSessionManager struct {
	logger   *slog.Logger
	mu       sync.RWMutex
	Streams  map[StreamKey]*StreamInfo
	Sessions ForwardSessionSet
}

type ForwardSessionFactory func(fsm *ForwardSessionManager, key StreamKey, streamID string, handlerType string, filter string, cfg map[string]any) (fs ForwardSessionChannel, err error)

var NullStreamKey = internal.NullStreamKey
var ForwardSessionTypes = make(map[string]ForwardSessionFactory)

func (fsm *ForwardSessionManager) Logger() *slog.Logger {
	return fsm.logger
}

func (fsm *ForwardSessionManager) Lock() {
	fsm.mu.Lock()
}

func (fsm *ForwardSessionManager) Unlock() {
	fsm.mu.Unlock()
}

func (fsm *ForwardSessionManager) RLock() {
	fsm.mu.RLock()
}

func (fsm *ForwardSessionManager) RUnlock() {
	fsm.mu.RUnlock()
}

// NewForwardSessionManager creates a new ForwardSessionManager
func NewForwardSessionManager(logger *slog.Logger) *ForwardSessionManager {
	return &ForwardSessionManager{
		logger:   logger,
		mu:       sync.RWMutex{},
		Streams:  make(map[StreamKey]*StreamInfo),
		Sessions: make(ForwardSessionSet),
	}
}

func (fsm *ForwardSessionManager) GetStream(key StreamKey) (si *StreamInfo, ok bool) {
	fsm.RLock()
	s, ok := fsm.Streams[key]
	fsm.RUnlock()
	return s, ok
}

func (fsm *ForwardSessionManager) GetStreamByID(id string) (si *StreamInfo, key StreamKey) {
	fsm.RLock()
	defer fsm.RUnlock()
	for k, s := range fsm.Streams {
		if s.ID == id {
			return s, k
		}
	}
	return nil, NullStreamKey
}

// UpdateStream adds a discovered stream to the streams registry
func (fsm *ForwardSessionManager) UpdateStream(key StreamKey, t time.Time, bytes int) (si *StreamInfo) {
	fsm.Lock()
	si, exists := fsm.Streams[key]
	defer fsm.Unlock()

	if exists {
		si.LastSeen = t
		si.Packets++
		si.Bytes += uint64(bytes)
		return si
	}

	si = &StreamInfo{
		ID:              rand.Text(),
		SrcIP:           key.SrcIP,
		ErspanID:        key.ErspanID,
		FirstSeen:       t,
		LastSeen:        t,
		Packets:         1,
		Bytes:           uint64(bytes),
		ForwardSessions: make(internal.ForwardSessionSet),
	}
	fsm.Streams[key] = si
	fsm.logger.Info("registered new stream", "stream_id", si.ID, "key", key.String())
	// TODO: Link any existing sessions for this stream
	return si
}

func RegisterForwardSessionType(name string, factory ForwardSessionFactory) {
	ForwardSessionTypes[name] = factory
}
