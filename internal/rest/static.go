package rest

import (
	"embed"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed static/*.html
var content embed.FS

// Setup static file server and routes
func setupStatic(r *chi.Mux) {
	fs := http.FileServer(http.FS(content))
	r.Handle("/static/*", fs)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "static/stream_dashboard.html", http.StatusSeeOther)
	})
}
