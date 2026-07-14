package api

import (
	"net/http"

	"github.com/pixll/atlas/build"
	"github.com/pixll/atlas/github"
	"github.com/pixll/atlas/store"
)

type Server struct {
	addr string
	mux  *http.ServeMux
}

// Options configures the Atlas API server.
type Options struct {
	Addr              string
	Store             *store.Store
	WebhookSecret     string
	GitHub            *github.Client
	WebhookPublicURL  string
	WorkerConfig      build.WorkerConfig
	Deployer          build.Deployer
	Status            StatusConfig
}

func New(opts Options) *Server {
	mux := http.NewServeMux()
	s := &Server{addr: opts.Addr, mux: mux}

	worker := build.NewWorker(opts.Store, opts.WorkerConfig, opts.Deployer, opts.GitHub)

	mux.HandleFunc("GET /healthz", s.handleHealthz)
	RegisterStatus(mux, opts.Status, opts.Store)
	RegisterApps(mux, opts.Store, opts.Deployer, opts.WorkerConfig.Namespace)
	RegisterRepos(mux, opts.Store, opts.GitHub)
	RegisterBuilds(mux, opts.Store, worker)
	RegisterWebhooks(mux, opts.Store, opts.WebhookSecret, worker)
	RegisterGitHubAuth(mux, opts.GitHub, opts.Store, opts.WebhookSecret)
	RegisterGitHubAPI(mux, opts.GitHub, opts.Store)
	RegisterUI(mux)

	return s
}

func (s *Server) Run() error {
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
