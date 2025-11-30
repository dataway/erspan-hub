package rest

import (
	"embed"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed static/*.html
var content embed.FS

// Setup static file server and routes
func setupStatic(r *chi.Mux, prefix string) {
	fs := http.FileServer(http.FS(content))
	r.Handle(prefix+"/static/*", http.StripPrefix(prefix, fs))
	r.Get(prefix, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, prefix+"/static/stream_dashboard.html", http.StatusSeeOther)
	})
}
