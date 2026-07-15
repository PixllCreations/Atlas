package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/pixll/atlas/app"
	"github.com/pixll/atlas/plan"
	"github.com/pixll/atlas/runtime"
	"github.com/pixll/atlas/store"
)

type workloadResponse struct {
	Name      string `json:"name"`
	Component string `json:"component"`
	Type      string `json:"type,omitempty"`
	Ready     bool   `json:"ready"`
	Replicas  int32  `json:"replicas,omitempty"`
	Source    string `json:"source,omitempty"` // "live" | "snapshot"
}

// WorkloadRuntime lists and streams project workload logs.
type WorkloadRuntime interface {
	ListManagedWorkloads(ctx context.Context, namespace, projectID string) ([]runtime.Workload, error)
	FollowWorkloadLogs(ctx context.Context, namespace, name string, tailLines int64) (*runtime.LogStream, error)
	SnapshotWorkloadLogs(ctx context.Context, namespace, name string, tailLines int64) (log string, pod string, err error)
}

type workloadsHandler struct {
	store *store.Store
	rt    WorkloadRuntime
}

func RegisterWorkloads(mux *http.ServeMux, st *store.Store, rt WorkloadRuntime) {
	h := &workloadsHandler{store: st, rt: rt}
	mux.HandleFunc("GET /apps/{id}/workloads", h.list)
	mux.HandleFunc("GET /apps/{id}/workloads/{name}/logs", h.logs)
}

func (h *workloadsHandler) list(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadApp(w, r)
	if !ok {
		return
	}
	ns := plan.NamespaceName(a.Name)

	var resp []workloadResponse
	if h.rt != nil {
		live, err := h.rt.ListManagedWorkloads(r.Context(), ns, a.ID)
		if err == nil && len(live) > 0 {
			resp = make([]workloadResponse, 0, len(live))
			for _, wl := range live {
				resp = append(resp, workloadResponse{
					Name:      wl.Name,
					Component: wl.Component,
					Type:      wl.Type,
					Ready:     wl.Ready,
					Replicas:  wl.Replicas,
					Source:    "live",
				})
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}
	}

	resp = workloadsFromSnapshot(a)
	writeJSON(w, http.StatusOK, resp)
}

func (h *workloadsHandler) logs(w http.ResponseWriter, r *http.Request) {
	a, ok := h.loadApp(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "workload name is required")
		return
	}
	if h.rt == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes runtime unavailable")
		return
	}

	ns := plan.NamespaceName(a.Name)
	if !h.canAccessWorkload(r.Context(), a, ns, name) {
		writeError(w, http.StatusNotFound, "workload not found")
		return
	}

	tail := int64(200)
	if v := r.URL.Query().Get("tailLines"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			tail = n
		}
	}
	follow := r.URL.Query().Get("follow") == "1" || r.URL.Query().Get("follow") == "true"

	if !follow {
		logText, pod, err := h.rt.SnapshotWorkloadLogs(r.Context(), ns, name, tail)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"workload": name,
			"pod":      pod,
			"log":      logText,
		})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ls, err := h.rt.FollowWorkloadLogs(r.Context(), ns, name, tail)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer ls.Stream.Close()

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
		"workload":  name,
		"pod":       ls.Pod,
		"container": ls.Container,
	})

	buf := make([]byte, 4096)
	for {
		n, readErr := ls.Stream.Read(buf)
		if n > 0 {
			writeSSE("log", map[string]string{"chunk": string(buf[:n])})
		}
		if readErr == io.EOF {
			writeSSE("done", map[string]string{"status": "eof"})
			return
		}
		if readErr != nil {
			if r.Context().Err() != nil {
				return
			}
			writeSSE("error", map[string]string{"error": readErr.Error()})
			return
		}
	}
}

func (h *workloadsHandler) loadApp(w http.ResponseWriter, r *http.Request) (app.App, bool) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return app.App{}, false
	}
	a, err := h.store.GetApp(r.Context(), id)
	if err != nil {
		writeAppLookupError(w, err)
		return app.App{}, false
	}
	return a, true
}

func workloadsFromSnapshot(a app.App) []workloadResponse {
	resp := []workloadResponse{{
		Name:      plan.ApplicationName,
		Component: runtime.ComponentApplication,
		Type:      "app",
		Ready:     false,
		Source:    "snapshot",
	}}
	if len(a.DeploymentSnapshot) == 0 {
		return resp
	}
	var snap struct {
		App struct {
			Name string `json:"name"`
		} `json:"app"`
		Dependencies []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(a.DeploymentSnapshot, &snap); err != nil {
		return resp
	}
	if snap.App.Name != "" {
		resp[0].Name = snap.App.Name
	}
	for _, d := range snap.Dependencies {
		if d.Name == "" {
			continue
		}
		resp = append(resp, workloadResponse{
			Name:      d.Name,
			Component: runtime.ComponentDependency,
			Type:      d.Type,
			Source:    "snapshot",
		})
	}
	return resp
}

func workloadAllowed(a app.App, name string) bool {
	for _, w := range workloadsFromSnapshot(a) {
		if w.Name == name {
			return true
		}
	}
	return name == plan.ApplicationName
}

func (h *workloadsHandler) canAccessWorkload(ctx context.Context, a app.App, ns, name string) bool {
	if h.rt != nil {
		live, err := h.rt.ListManagedWorkloads(ctx, ns, a.ID)
		if err == nil && len(live) > 0 {
			for _, w := range live {
				if w.Name == name {
					return true
				}
			}
			return false
		}
	}
	return workloadAllowed(a, name)
}
