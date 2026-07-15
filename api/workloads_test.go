package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/pixll/atlas/runtime"
	"github.com/pixll/atlas/store"
)

func openWorkloadsTestServer(t *testing.T, rt WorkloadRuntime) (*store.Store, *httptest.Server) {
	t.Helper()

	dsn := os.Getenv("ATLAS_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://atlas:atlas@localhost:5432/atlas?sslmode=disable"
	}

	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	ensureMigrations(t, ctx, dsn)

	mux := http.NewServeMux()
	RegisterApps(mux, st, nil, "")
	RegisterWorkloads(mux, st, rt)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return st, ts
}

func TestListWorkloads_FromSnapshot(t *testing.T) {
	st, ts := openWorkloadsTestServer(t, nil)

	name := "wl-snap-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)

	snap, _ := json.Marshal(map[string]any{
		"namespace": "atlas-" + name,
		"app":       map[string]any{"name": "app", "port": 8080},
		"dependencies": []map[string]any{
			{"name": "redis", "type": "redis", "endpoint": "redis:6379"},
		},
	})
	if err := st.UpdateAppDeploymentSnapshot(context.Background(), app.ID, snap); err != nil {
		t.Fatal(err)
	}

	res, err := http.Get(ts.URL + "/apps/" + app.ID + "/workloads")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var got []workloadResponse
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("workloads = %+v", got)
	}
	if got[0].Name != "app" || got[1].Name != "redis" {
		t.Fatalf("names = %+v", got)
	}
	if got[0].Source != "snapshot" {
		t.Fatalf("source = %q", got[0].Source)
	}
}

func TestWorkloadLogs_UnavailableWithoutRuntime(t *testing.T) {
	_, ts := openWorkloadsTestServer(t, nil)

	name := "wl-norun-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)

	res, err := http.Get(ts.URL + "/apps/" + app.ID + "/workloads/app/logs?follow=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestWorkloadLogs_RejectsUnknownName(t *testing.T) {
	_, ts := openWorkloadsTestServer(t, &fakeWorkloadRuntime{})

	name := "wl-bad-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)

	res, err := http.Get(ts.URL + "/apps/" + app.ID + "/workloads/not-a-service/logs")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

type fakeWorkloadRuntime struct{}

func (f *fakeWorkloadRuntime) ListManagedWorkloads(context.Context, string, string) ([]runtime.Workload, error) {
	return nil, nil
}

func (f *fakeWorkloadRuntime) FollowWorkloadLogs(context.Context, string, string, int64) (*runtime.LogStream, error) {
	return nil, nil
}

func (f *fakeWorkloadRuntime) SnapshotWorkloadLogs(context.Context, string, string, int64) (string, string, error) {
	return "", "", nil
}
