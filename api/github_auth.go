package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/pixll/atlas/github"
	"github.com/pixll/atlas/store"
)

type githubAuthHandler struct {
	client   *github.Client
	store    *store.Store
	stateKey string
}

func RegisterGitHubAuth(mux *http.ServeMux, client *github.Client, st *store.Store, stateKey string) {
	if client == nil {
		return
	}
	h := &githubAuthHandler{
		client:   client,
		store:    st,
		stateKey: stateKey,
	}
	mux.HandleFunc("GET /auth/github/install", h.install)
	mux.HandleFunc("GET /auth/github/callback", h.callback)
}

func (h *githubAuthHandler) install(w http.ResponseWriter, r *http.Request) {
	returnPath := strings.TrimSpace(r.URL.Query().Get("return"))
	if returnPath == "" {
		returnPath = "/"
	}
	state, err := github.SignInstallState(h.stateKey, returnPath, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start install")
		return
	}
	http.Redirect(w, r, h.client.InstallURL(state), http.StatusFound)
}

func (h *githubAuthHandler) callback(w http.ResponseWriter, r *http.Request) {
	returnPath := "/"
	if state := r.URL.Query().Get("state"); state != "" {
		if path, err := github.VerifyInstallState(h.stateKey, state, time.Now()); err == nil {
			returnPath = path
		} else {
			log.Printf("github callback: invalid state, falling back to /: %v", err)
		}
	}

	installationID, err := parseInstallationID(r.URL.Query().Get("installation_id"))
	if err != nil || installationID == 0 {
		writeError(w, http.StatusBadRequest, "missing installation_id")
		return
	}

	inst, err := h.client.GetInstallation(r.Context(), installationID)
	if err != nil {
		log.Printf("github callback: get installation %d: %v", installationID, err)
		writeError(w, http.StatusBadGateway, "failed to load installation")
		return
	}
	if err := h.store.UpsertInstallation(r.Context(), store.GitHubInstallation{
		ID:           inst.ID,
		AccountLogin: inst.AccountLogin,
		AccountType:  inst.AccountType,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save installation")
		return
	}

	http.Redirect(w, r, returnPath, http.StatusFound)
}
