package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/pixll/atlas/build"
	"github.com/pixll/atlas/store"
	"github.com/pixll/atlas/webhook"
)

const maxWebhookBody = 1 << 20 // 1 MiB

type webhooksHandler struct {
	store  *store.Store
	secret string
	worker *build.Worker
}

func RegisterWebhooks(mux *http.ServeMux, st *store.Store, secret string, worker *build.Worker) {
	h := &webhooksHandler{store: st, secret: secret, worker: worker}
	mux.HandleFunc("POST /webhooks/github", h.github)
}

func (h *webhooksHandler) github(w http.ResponseWriter, r *http.Request) {
	eventType := r.Header.Get("X-GitHub-Event")

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	sig := r.Header.Get(webhook.GitHubSignatureHeader)
	if err := webhook.VerifyGitHubSignature(h.secret, body, sig); err != nil {
		if errors.Is(err, webhook.ErrInvalidSignature) {
			writeError(w, http.StatusUnauthorized, "invalid signature")
			return
		}
		writeError(w, http.StatusInternalServerError, "webhook not configured")
		return
	}

	switch eventType {
	case "push":
		h.handlePush(w, r, body)
	case "installation":
		h.handleInstallation(w, r, body)
	case "installation_repositories":
		h.handleInstallationRepositories(w, body)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *webhooksHandler) handlePush(w http.ResponseWriter, r *http.Request, body []byte) {
	event, err := webhook.ParseGitHubPush(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	branch, err := webhook.BranchFromRef(event.Ref)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	appID, err := h.findAppForPush(r.Context(), event, branch)
	if errors.Is(err, store.ErrRepoNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to lookup app")
		return
	}

	b, err := h.store.CreateBuild(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create build")
		return
	}

	if h.worker != nil {
		go func(buildID string) {
			if err := h.worker.Process(context.Background(), buildID); err != nil {
				log.Printf("process build %s: %v", buildID, err)
			}
		}(b.ID)
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":   "accepted",
		"app_id":   appID,
		"build_id": b.ID,
	})
}

func (h *webhooksHandler) findAppForPush(ctx context.Context, event webhook.GitHubPushEvent, branch string) (string, error) {
	if event.Repository.ID != 0 {
		appID, err := h.store.FindAppByGitHubRepoID(ctx, event.Repository.ID, branch)
		if err == nil {
			return appID, nil
		}
		if !errors.Is(err, store.ErrRepoNotFound) {
			return "", err
		}
	}
	return h.store.FindAppByRepo(ctx, event.Repository.HTMLURL, branch)
}

func (h *webhooksHandler) handleInstallation(w http.ResponseWriter, r *http.Request, body []byte) {
	var event struct {
		Action       string `json:"action"`
		Installation struct {
			ID      int64 `json:"id"`
			Account struct {
				Login string `json:"login"`
				Type  string `json:"type"`
			} `json:"account"`
		} `json:"installation"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	switch event.Action {
	case "created", "added":
		if event.Installation.ID == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := h.store.UpsertInstallation(r.Context(), store.GitHubInstallation{
			ID:           event.Installation.ID,
			AccountLogin: event.Installation.Account.Login,
			AccountType:  event.Installation.Account.Type,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save installation")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *webhooksHandler) handleInstallationRepositories(w http.ResponseWriter, body []byte) {
	event, err := webhook.ParseInstallationRepositories(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if event.Action != "removed" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var ids []int64
	for _, repo := range event.Repositories {
		if repo.ID != 0 {
			ids = append(ids, repo.ID)
		}
	}
	if err := h.store.UnlinkReposByGitHubIDs(context.Background(), ids); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink repos")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
