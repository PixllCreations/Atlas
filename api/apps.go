package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/pixll/atlas/app"
	"github.com/pixll/atlas/build"
	"github.com/pixll/atlas/store"
)

type createAppRequest struct {
	Name string `json:"name"`
	Port *int   `json:"port"`
}

type updateAppRequest struct {
	Port *int `json:"port"`
}

type appResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Port      int    `json:"port"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type appsHandler struct {
	store     *store.Store
	deployer  build.Deployer
	namespace string
}

func RegisterApps(mux *http.ServeMux, st *store.Store, deployer build.Deployer, namespace string) {
	h := &appsHandler{store: st, deployer: deployer, namespace: namespace}
	mux.HandleFunc("GET /apps", h.list)
	mux.HandleFunc("POST /apps", h.create)
	mux.HandleFunc("GET /apps/{id}", h.get)
	mux.HandleFunc("PATCH /apps/{id}", h.update)
	mux.HandleFunc("DELETE /apps/{id}", h.delete)
}

func (h *appsHandler) list(w http.ResponseWriter, r *http.Request) {
	apps, err := h.store.ListApps(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list apps")
		return
	}

	resp := make([]appResponse, 0, len(apps))
	for _, a := range apps {
		resp = append(resp, toAppResponse(a))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *appsHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	a, err := h.store.CreateApp(r.Context(), req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create app")
		return
	}
	if req.Port != nil {
		a, err = h.store.UpdateAppPort(r.Context(), a.ID, *req.Port)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid port")
			return
		}
	}
	writeJSON(w, http.StatusCreated, toAppResponse(a))
}

func (h *appsHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	a, err := h.store.GetApp(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get app")
		return
	}
	writeJSON(w, http.StatusOK, toAppResponse(a))
}

func (h *appsHandler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var req updateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Port == nil {
		writeError(w, http.StatusBadRequest, "port is required")
		return
	}

	a, err := h.store.UpdateAppPort(r.Context(), id, *req.Port)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid port")
		return
	}
	writeJSON(w, http.StatusOK, toAppResponse(a))
}

func (h *appsHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	a, err := h.store.GetApp(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get app")
		return
	}

	if h.deployer != nil {
		ns := h.namespace
		if ns == "" {
			ns = "default"
		}
		if err := h.deployer.DeleteIngress(r.Context(), ns, a.Name); err != nil {
			log.Printf("teardown ingress %s: %v", a.Name, err)
			writeError(w, http.StatusInternalServerError, "failed to teardown app")
			return
		}
		if err := h.deployer.DeleteService(r.Context(), ns, a.Name); err != nil {
			log.Printf("teardown service %s: %v", a.Name, err)
			writeError(w, http.StatusInternalServerError, "failed to teardown app")
			return
		}
		if err := h.deployer.DeleteDeployment(r.Context(), ns, a.Name); err != nil {
			log.Printf("teardown deployment %s: %v", a.Name, err)
			writeError(w, http.StatusInternalServerError, "failed to teardown app")
			return
		}
	}

	err = h.store.DeleteApp(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete app")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toAppResponse(a app.App) appResponse {
	port := a.Port
	if port == 0 {
		port = app.DefaultPort
	}
	return appResponse{
		ID:        a.ID,
		Name:      a.Name,
		Port:      port,
		CreatedAt: a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: a.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
