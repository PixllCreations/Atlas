package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/pixll/atlas/app"
	"github.com/pixll/atlas/store"
)

type linkRepoRequest struct {
	URL      string `json:"url"`
	Provider string `json:"provider"`
	Branch   string `json:"branch"`
}

type repoResponse struct {
	URL      string `json:"url"`
	Provider string `json:"provider"`
	Branch   string `json:"branch"`
}

type reposHandler struct {
	store *store.Store
}

// RegisterRepos mounts repository link routes on mux.
func RegisterRepos(mux *http.ServeMux, st *store.Store) {
	h := &reposHandler{store: st}
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
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
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

	branch := req.Branch
	if branch == "" {
		branch = "main"
	}

	repo, err := h.store.LinkRepo(r.Context(), appID, app.Repo{
		URL:      req.URL,
		Provider: provider,
		Branch:   branch,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link repo")
		return
	}
	writeJSON(w, http.StatusOK, toRepoResponse(repo))
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
		URL:      repo.URL,
		Provider: string(repo.Provider),
		Branch:   repo.Branch,
	}
}
