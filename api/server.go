package api

import (
	"net/http"

	"github.com/pixll/atlas/build"
	"github.com/pixll/atlas/store"
)

type Server struct {
	addr string
	mux  *http.ServeMux
}

func New(addr string, st *store.Store, webhookSecret string) *Server {
	mux := http.NewServeMux()
	s := &Server{addr: addr, mux: mux}
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	RegisterApps(mux, st)
	RegisterRepos(mux, st)
	RegisterBuilds(mux, st)
	RegisterWebhooks(mux, st, webhookSecret, build.NewWorker(st))
	return s
}

func (s *Server) Run() error {
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
