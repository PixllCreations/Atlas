package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/pixll/atlas/app"
	"github.com/pixll/atlas/build"
	"github.com/pixll/atlas/plan"
	"github.com/pixll/atlas/store"
)

type createAppRequest struct {
	Name string `json:"name"`
}

type appResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type infrastructureDependency struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Endpoint string `json:"endpoint,omitempty"`
	Status   string `json:"status,omitempty"`
}

type infrastructureResponse struct {
	Namespace    string                     `json:"namespace"`
	Host         string                     `json:"host,omitempty"`
	AppName      string                     `json:"app_name"`
	AppPort      int                        `json:"app_port"`
	Dependencies []infrastructureDependency `json:"dependencies"`
}

type appsHandler struct {
	store     *store.Store
	deployer  build.Deployer
	namespace string // system/build namespace for legacy teardown
}

func RegisterApps(mux *http.ServeMux, st *store.Store, deployer build.Deployer, namespace string) {
	h := &appsHandler{store: st, deployer: deployer, namespace: namespace}
	mux.HandleFunc("GET /apps", h.list)
	mux.HandleFunc("POST /apps", h.create)
	mux.HandleFunc("GET /apps/{id}", h.get)
	mux.HandleFunc("DELETE /apps/{id}", h.delete)
	mux.HandleFunc("GET /apps/{id}/infrastructure", h.infrastructure)
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
		projectNS := plan.NamespaceName(a.Name)
		if err := h.deployer.DeleteNamespace(r.Context(), projectNS); err != nil {
			log.Printf("teardown namespace %s: %v", projectNS, err)
			writeError(w, http.StatusInternalServerError, "failed to teardown app")
			return
		}

		// Best-effort cleanup of pre-namespace resources in the system namespace.
		sysNS := h.namespace
		if sysNS == "" {
			sysNS = "default"
		}
		if err := h.deployer.DeleteIngress(r.Context(), sysNS, a.Name); err != nil {
			log.Printf("legacy teardown ingress %s: %v", a.Name, err)
		}
		if err := h.deployer.DeleteService(r.Context(), sysNS, a.Name); err != nil {
			log.Printf("legacy teardown service %s: %v", a.Name, err)
		}
		if err := h.deployer.DeleteDeployment(r.Context(), sysNS, a.Name); err != nil {
			log.Printf("legacy teardown deployment %s: %v", a.Name, err)
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

func (h *appsHandler) infrastructure(w http.ResponseWriter, r *http.Request) {
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

	resp := infrastructureResponse{
		Namespace:    plan.NamespaceName(a.Name),
		AppName:      plan.ApplicationName,
		Dependencies: []infrastructureDependency{},
	}

	if len(a.DeploymentSnapshot) > 0 {
		var snap struct {
			Namespace string `json:"namespace"`
			Host      string `json:"host"`
			App       struct {
				Name string `json:"name"`
				Port int    `json:"port"`
			} `json:"app"`
			Dependencies []struct {
				Name     string `json:"name"`
				Type     string `json:"type"`
				Endpoint string `json:"endpoint"`
			} `json:"dependencies"`
		}
		if err := json.Unmarshal(a.DeploymentSnapshot, &snap); err == nil {
			if snap.Namespace != "" {
				resp.Namespace = snap.Namespace
			}
			resp.Host = snap.Host
			if snap.App.Name != "" {
				resp.AppName = snap.App.Name
			}
			if snap.App.Port != 0 {
				resp.AppPort = snap.App.Port
			}
			for _, d := range snap.Dependencies {
				resp.Dependencies = append(resp.Dependencies, infrastructureDependency{
					Name:     d.Name,
					Type:     d.Type,
					Endpoint: d.Endpoint,
					Status:   "Declared",
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func toAppResponse(a app.App) appResponse {
	return appResponse{
		ID:        a.ID,
		Name:      a.Name,
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
