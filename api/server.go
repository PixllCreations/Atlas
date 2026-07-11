package api

import (
	"net/http"

	"github.com/pixll/atlas/store"
)

type Server struct {
	addr string
	mux  *http.ServeMux
}

// New creates a Server listening on addr with the given store for persistence.
func New(addr string, st *store.Store) *Server {
	mux := http.NewServeMux()
	s := &Server{addr: addr, mux: mux}
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	RegisterApps(mux, st)
	return s
}

func (s *Server) Run() error {
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
