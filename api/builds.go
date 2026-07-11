package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/pixll/atlas/build"
	"github.com/pixll/atlas/store"
)

type buildResponse struct {
	ID        string `json:"id"`
	AppID     string `json:"app_id"`
	Status    string `json:"status"`
	Image     string `json:"image"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type buildsHandler struct {
	store *store.Store
}

func RegisterBuilds(mux *http.ServeMux, st *store.Store) {
	h := &buildsHandler{store: st}
	mux.HandleFunc("GET /apps/{id}/builds", h.list)
	mux.HandleFunc("GET /apps/{id}/builds/{build_id}", h.get)
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

func (h *buildsHandler) get(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.ensureAppExists(r.Context(), appID); err != nil {
		writeAppLookupError(w, err)
		return
	}

	buildID := r.PathValue("build_id")
	if buildID == "" {
		writeError(w, http.StatusBadRequest, "build_id is required")
		return
	}

	b, err := h.store.GetBuild(r.Context(), buildID)
	if errors.Is(err, store.ErrBuildNotFound) {
		writeError(w, http.StatusNotFound, "build not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get build")
		return
	}
	if b.AppID != appID {
		writeError(w, http.StatusNotFound, "build not found")
		return
	}

	writeJSON(w, http.StatusOK, toBuildResponse(b))
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
		Image:     b.Image,
		CreatedAt: b.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: b.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}
