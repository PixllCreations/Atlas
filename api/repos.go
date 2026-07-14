package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/pixll/atlas/app"
	"github.com/pixll/atlas/github"
	"github.com/pixll/atlas/store"
)

type linkRepoRequest struct {
	URL            string `json:"url"`
	Provider       string `json:"provider"`
	Branch         string `json:"branch"`
	GitHubFullName string `json:"github_full_name"`
	InstallationID int64  `json:"installation_id"`
}

type repoResponse struct {
	URL            string `json:"url"`
	Provider       string `json:"provider"`
	Branch         string `json:"branch"`
	GitHubFullName string `json:"github_full_name,omitempty"`
	InstallationID int64  `json:"installation_id,omitempty"`
}

type reposHandler struct {
	store  *store.Store
	github *github.Client
}

func RegisterRepos(mux *http.ServeMux, st *store.Store, gh *github.Client) {
	h := &reposHandler{store: st, github: gh}
	mux.HandleFunc("PUT /apps/{id}/repo", h.link)
	mux.HandleFunc("GET /apps/{id}/repo", h.get)
	mux.HandleFunc("DELETE /apps/{id}/repo", h.unlink)
}

func (h *reposHandler) link(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.ensureAppExists(r.Context(), appID); err != nil {
		writeAppLookupError(w, err)
		return
	}

	var req linkRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	provider := app.Provider(req.Provider)
	if provider == "" {
		provider = app.ProviderGitHub
	}
	if provider != app.ProviderGitHub {
		writeError(w, http.StatusBadRequest, "unsupported provider")
		return
	}

	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = "main"
	}

	repo, err := h.resolveRepo(r.Context(), req, provider, branch)
	if err != nil {
		writeRepoLinkError(w, err)
		return
	}

	linked, err := h.store.LinkRepo(r.Context(), appID, repo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link repo")
		return
	}
	writeJSON(w, http.StatusOK, toRepoResponse(linked))
}

func (h *reposHandler) resolveRepo(ctx context.Context, req linkRepoRequest, provider app.Provider, branch string) (app.Repo, error) {
	fullName := strings.TrimSpace(req.GitHubFullName)
	if fullName != "" {
		if h.github == nil {
			return app.Repo{}, errGitHubAppNotConfigured
		}
		if req.InstallationID == 0 {
			return app.Repo{}, errInstallationRequired
		}
		if _, err := h.store.GetInstallation(ctx, req.InstallationID); errors.Is(err, store.ErrNotFound) {
			return app.Repo{}, errInstallationNotFound
		} else if err != nil {
			return app.Repo{}, err
		}
		ghRepo, err := h.github.GetRepository(ctx, req.InstallationID, fullName)
		if err != nil {
			return app.Repo{}, errGitHubRepoNotFound
		}
		if branch == "main" && ghRepo.DefaultBranch != "" {
			branch = ghRepo.DefaultBranch
		}
		return app.Repo{
			URL:            ghRepo.HTMLURL,
			Provider:       provider,
			Branch:         branch,
			GitHubRepoID:   ghRepo.ID,
			GitHubFullName: ghRepo.FullName,
			InstallationID: req.InstallationID,
		}, nil
	}

	url := strings.TrimSpace(req.URL)
	if url == "" {
		return app.Repo{}, errRepoURLRequired
	}
	return app.Repo{
		URL:      url,
		Provider: provider,
		Branch:   branch,
	}, nil
}

var (
	errRepoURLRequired          = errors.New("url or github_full_name is required")
	errGitHubAppNotConfigured   = errors.New("github app not configured")
	errInstallationRequired     = errors.New("installation_id is required")
	errInstallationNotFound     = errors.New("installation not found")
	errGitHubRepoNotFound       = errors.New("repository not found")
)

func writeRepoLinkError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errRepoURLRequired):
		writeError(w, http.StatusBadRequest, "url or github_full_name is required")
	case errors.Is(err, errGitHubAppNotConfigured):
		writeError(w, http.StatusBadRequest, "github app not configured")
	case errors.Is(err, errInstallationRequired):
		writeError(w, http.StatusBadRequest, "installation_id is required")
	case errors.Is(err, errInstallationNotFound):
		writeError(w, http.StatusNotFound, "installation not found")
	case errors.Is(err, errGitHubRepoNotFound):
		writeError(w, http.StatusNotFound, "repository not found")
	default:
		writeError(w, http.StatusInternalServerError, "failed to resolve repository")
	}
}

func (h *reposHandler) get(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.ensureAppExists(r.Context(), appID); err != nil {
		writeAppLookupError(w, err)
		return
	}

	repo, err := h.store.GetRepo(r.Context(), appID)
	if errors.Is(err, store.ErrRepoNotFound) {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get repo")
		return
	}
	writeJSON(w, http.StatusOK, toRepoResponse(repo))
}

func (h *reposHandler) unlink(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.ensureAppExists(r.Context(), appID); err != nil {
		writeAppLookupError(w, err)
		return
	}

	err := h.store.UnlinkRepo(r.Context(), appID)
	if errors.Is(err, store.ErrRepoNotFound) {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink repo")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *reposHandler) ensureAppExists(ctx context.Context, appID string) error {
	_, err := h.store.GetApp(ctx, appID)
	return err
}

func writeAppLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "failed to get app")
}

func toRepoResponse(repo app.Repo) repoResponse {
	return repoResponse{
		URL:            repo.URL,
		Provider:       string(repo.Provider),
		Branch:         repo.Branch,
		GitHubFullName: repo.GitHubFullName,
		InstallationID: repo.InstallationID,
	}
}
