package api

import (
	"context"
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
	if r.Header.Get("X-GitHub-Event") != "push" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

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

	appID, err := h.store.FindAppByRepo(r.Context(), event.Repository.HTMLURL, branch)
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
