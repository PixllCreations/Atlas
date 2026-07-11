package api

import (
	"net/http"
)

type Server struct {
	addr string
	mux  *http.ServeMux
}

func New(addr string) *Server {
	mux := http.NewServeMux()
	s := &Server{addr: addr, mux: mux}
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	return s
}

func (s *Server) Run() error {
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
