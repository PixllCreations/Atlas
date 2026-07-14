package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/pixll/atlas/github"
	"github.com/pixll/atlas/store"
)

type githubAPIHandler struct {
	client *github.Client
	store  *store.Store
}

type githubRepoResponse struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
}

func RegisterGitHubAPI(mux *http.ServeMux, client *github.Client, st *store.Store) {
	if client == nil {
		return
	}
	h := &githubAPIHandler{client: client, store: st}
	mux.HandleFunc("GET /github/installations", h.listInstallations)
	mux.HandleFunc("POST /github/installations/sync", h.syncInstallations)
	mux.HandleFunc("GET /github/installations/{id}/repos", h.listRepos)
}

func (h *githubAPIHandler) listInstallations(w http.ResponseWriter, r *http.Request) {
	insts, err := h.store.ListInstallations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list installations")
		return
	}
	resp := make([]InstallationResponse, 0, len(insts))
	for _, inst := range insts {
		resp = append(resp, InstallationResponse{
			ID:           inst.ID,
			AccountLogin: inst.AccountLogin,
			AccountType:  inst.AccountType,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *githubAPIHandler) syncInstallations(w http.ResponseWriter, r *http.Request) {
	remote, err := h.client.ListAppInstallations(r.Context())
	if err != nil {
		log.Printf("sync github installations: %v", err)
		writeError(w, http.StatusBadGateway, "failed to sync installations")
		return
	}
	for _, inst := range remote {
		if err := h.store.UpsertInstallation(r.Context(), store.GitHubInstallation{
			ID:           inst.ID,
			AccountLogin: inst.AccountLogin,
			AccountType:  inst.AccountType,
		}); err != nil {
			log.Printf("upsert installation %d: %v", inst.ID, err)
			writeError(w, http.StatusInternalServerError, "failed to save installations")
			return
		}
	}
	h.listInstallations(w, r)
}

func (h *githubAPIHandler) listRepos(w http.ResponseWriter, r *http.Request) {
	installationID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || installationID == 0 {
		writeError(w, http.StatusBadRequest, "invalid installation id")
		return
	}
	if _, err := h.store.GetInstallation(r.Context(), installationID); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "installation not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load installation")
		return
	}

	repos, err := h.client.ListInstallationRepos(r.Context(), installationID)
	if err != nil {
		log.Printf("list installation %d repos: %v", installationID, err)
		writeError(w, http.StatusBadGateway, "failed to list repositories")
		return
	}
	resp := make([]githubRepoResponse, 0, len(repos))
	for _, repo := range repos {
		resp = append(resp, githubRepoResponse{
			ID:            repo.ID,
			FullName:      repo.FullName,
			HTMLURL:       repo.HTMLURL,
			Private:       repo.Private,
			DefaultBranch: repo.DefaultBranch,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
