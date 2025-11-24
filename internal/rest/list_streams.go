package rest

import (
	"encoding/json"
	"net/http"
	"time"

	"anthonyuk.dev/erspan-hub/internal/forward"
)

func (rsvr *RestServer) listStreamsHandler(w http.ResponseWriter, r *http.Request) {
	rsvr.fsm.RLock()
	defer rsvr.fsm.RUnlock()
	type out struct {
		ID         string              `json:"id"`
		StreamInfo *forward.StreamInfo `json:"stream_info"`
	}
	var list []out
	for k := range rsvr.fsm.Streams {
		list = append(list, out{k.String(), rsvr.fsm.Streams[k]})
	}
	json.NewEncoder(w).Encode(list)
}

func (rsvr *RestServer) listStreamsSseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "close")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ticker.C:
			w.Write([]byte("data: "))
			rsvr.listStreamsHandler(w, r)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}
