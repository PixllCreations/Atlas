package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/pixll/atlas/build"
	"github.com/pixll/atlas/store"
)

type buildResponse struct {
	ID        string `json:"id"`
	AppID     string `json:"app_id"`
	Status    string `json:"status"`
	Phase     string `json:"phase"`
	Image     string `json:"image"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type buildsHandler struct {
	store  *store.Store
	worker *build.Worker
}

func RegisterBuilds(mux *http.ServeMux, st *store.Store, worker *build.Worker) {
	h := &buildsHandler{store: st, worker: worker}
	mux.HandleFunc("GET /apps/{id}/builds", h.list)
	mux.HandleFunc("POST /apps/{id}/builds", h.create)
	mux.HandleFunc("GET /apps/{id}/builds/{build_id}", h.get)
	mux.HandleFunc("GET /apps/{id}/builds/{build_id}/logs", h.logs)
}

func (h *buildsHandler) list(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.ensureAppExists(r.Context(), appID); err != nil {
		writeAppLookupError(w, err)
		return
	}

	builds, err := h.store.ListBuildsByApp(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list builds")
		return
	}

	resp := make([]buildResponse, 0, len(builds))
	for _, b := range builds {
		resp = append(resp, toBuildResponse(b))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *buildsHandler) create(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.ensureAppExists(r.Context(), appID); err != nil {
		writeAppLookupError(w, err)
		return
	}

	if _, err := h.store.GetRepo(r.Context(), appID); errors.Is(err, store.ErrRepoNotFound) {
		writeError(w, http.StatusBadRequest, "repo not linked")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get repo")
		return
	}

	active, err := h.store.ListBuildsByApp(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list builds")
		return
	}
	for _, b := range active {
		if b.Status == build.StatusPending || b.Status == build.StatusRunning {
			writeError(w, http.StatusConflict, "a build is already in progress")
			return
		}
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

func (h *buildsHandler) get(w http.ResponseWriter, r *http.Request) {
	b, ok := h.loadBuild(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toBuildResponse(b))
}

func (h *buildsHandler) logs(w http.ResponseWriter, r *http.Request) {
	b, ok := h.loadBuild(w, r)
	if !ok {
		return
	}

	follow := r.URL.Query().Get("follow") == "1" || r.URL.Query().Get("follow") == "true"
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	if !follow {
		writeJSON(w, http.StatusOK, map[string]any{
			"build_id": b.ID,
			"status":   string(b.Status),
			"phase":    string(b.Phase),
			"log":      b.Log,
			"offset":   len(b.Log),
		})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	writeSSE := func(event string, payload any) {
		data, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	writeSSE("status", map[string]string{
		"status": string(b.Status),
		"phase":  string(b.Phase),
	})

	sent := offset
	if sent > len(b.Log) {
		sent = 0
	}
	if sent < len(b.Log) {
		writeSSE("log", map[string]any{
			"chunk":  b.Log[sent:],
			"offset": len(b.Log),
		})
		sent = len(b.Log)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			cur, err := h.store.GetBuild(r.Context(), b.ID)
			if err != nil {
				writeSSE("error", map[string]string{"error": "failed to read build"})
				return
			}
			if sent < len(cur.Log) {
				writeSSE("log", map[string]any{
					"chunk":  cur.Log[sent:],
					"offset": len(cur.Log),
				})
				sent = len(cur.Log)
			}
			writeSSE("status", map[string]string{
				"status": string(cur.Status),
				"phase":  string(cur.Phase),
			})
			if cur.Status == build.StatusSucceeded || cur.Status == build.StatusFailed {
				writeSSE("done", map[string]string{"status": string(cur.Status)})
				return
			}
		}
	}
}

func (h *buildsHandler) loadBuild(w http.ResponseWriter, r *http.Request) (build.Build, bool) {
	appID := r.PathValue("id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return build.Build{}, false
	}
	if err := h.ensureAppExists(r.Context(), appID); err != nil {
		writeAppLookupError(w, err)
		return build.Build{}, false
	}

	buildID := r.PathValue("build_id")
	if buildID == "" {
		writeError(w, http.StatusBadRequest, "build_id is required")
		return build.Build{}, false
	}

	b, err := h.store.GetBuild(r.Context(), buildID)
	if errors.Is(err, store.ErrBuildNotFound) {
		writeError(w, http.StatusNotFound, "build not found")
		return build.Build{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get build")
		return build.Build{}, false
	}
	if b.AppID != appID {
		writeError(w, http.StatusNotFound, "build not found")
		return build.Build{}, false
	}
	return b, true
}

func (h *buildsHandler) ensureAppExists(ctx context.Context, appID string) error {
	_, err := h.store.GetApp(ctx, appID)
	return err
}

func toBuildResponse(b build.Build) buildResponse {
	return buildResponse{
		ID:        b.ID,
		AppID:     b.AppID,
		Status:    string(b.Status),
		Phase:     string(b.Phase),
		Image:     b.Image,
		CreatedAt: b.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: b.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}
