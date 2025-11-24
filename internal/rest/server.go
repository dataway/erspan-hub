package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"anthonyuk.dev/erspan-hub/internal"
	"anthonyuk.dev/erspan-hub/internal/forward"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type RestServer struct {
	logger *slog.Logger
	config *Config
	fsm    *forward.ForwardSessionManager
}

func RunServer(cfg *Config, fsm *forward.ForwardSessionManager) error {

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(httplog.RequestLogger(fsm.Logger(), &httplog.Options{}))

	rsvr := &RestServer{
		logger: fsm.Logger(),
		config: cfg,
		fsm:    fsm,
	}

	setupStatic(r)

	// API routes
	r.Get("/streams", rsvr.listStreamsHandler)
	r.Get("/streams/sse", rsvr.listStreamsSseHandler)
	r.Post("/forward", rsvr.createForwardSessionHandler)
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/allocs", pprof.Handler("allocs").ServeHTTP)
	r.HandleFunc("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	r.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	r.HandleFunc("/debug/pprof/mutex", pprof.Handler("mutex").ServeHTTP)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.BindIP, cfg.Port),
		Handler:      r,
		WriteTimeout: 0, // disable timeout for SSE
		ReadTimeout:  3600 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// start server
	go func() {
		rsvr.logger.Info("▶️  HTTP server listening on " + srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			rsvr.logger.Error("HTTP server error", "error", err)
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	rsvr.logger.Info("🛑 Shutting down due to signal…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// forwardReq represents the JSON request payload for starting packet forwarding
type forwardReq struct {
	SrcIP        string         `json:"src_ip"`
	ErspanID     uint16         `json:"erspan_id"`
	StreamInfoID string         `json:"stream_info_id"`
	Type         string         `json:"type"`
	Filter       string         `json:"filter"`
	Config       map[string]any `json:"cfg"`
}

func (rsvr *RestServer) createForwardSessionHandler(w http.ResponseWriter, r *http.Request) {
	var req forwardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	rsvr.logger.Info("Received forward request", "src_ip", req.SrcIP, "erspan_id", req.ErspanID, "stream_info_id", req.StreamInfoID, "type", req.Type, "filter", req.Filter, "cfg", req.Config)
	si, err := rsvr.fsm.CreateForwardSessionByKey(
		internal.StreamKey{
			SrcIP:    internal.IPv4FromString(req.SrcIP),
			ErspanID: req.ErspanID,
		},
		req.Type, req.Filter, req.Config,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create forward session: %v", err), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(si)
}
