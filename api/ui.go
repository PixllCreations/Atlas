package api

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/pixll/atlas/web"
)

// RegisterUI serves the embedded SPA for non-API routes.
func RegisterUI(mux *http.ServeMux) {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic("web dist embed: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(dist))

	mux.Handle("GET /assets/", http.StripPrefix("/", fileServer))
	mux.HandleFunc("GET /{$}", spaIndex(dist))
	mux.HandleFunc("GET /{path...}", spaFallback(dist, fileServer))
}

func spaIndex(dist fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveIndexHTML(w, dist)
	}
}

func spaFallback(dist fs.FS, fileServer http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" && !strings.HasSuffix(path, "/") {
			if f, err := dist.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		serveIndexHTML(w, dist)
	}
}

func serveIndexHTML(w http.ResponseWriter, dist fs.FS) {
	f, err := dist.Open("index.html")
	if err != nil {
		http.Error(w, "ui not built", http.StatusServiceUnavailable)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, f)
}
