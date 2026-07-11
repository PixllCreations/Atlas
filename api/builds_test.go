package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/pixll/atlas/build"
	"github.com/pixll/atlas/store"
)

func testBuildsServer(t *testing.T, st *store.Store) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	RegisterApps(mux, st)
	RegisterBuilds(mux, st)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestListBuilds_Empty(t *testing.T) {
	_, ts := openBuildsTestServer(t)
	name := "builds-empty-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)

	builds := listBuilds(t, ts, app.ID)
	if len(builds) != 0 {
		t.Fatalf("list builds = %+v, want empty", builds)
	}
}

func TestListBuilds_AppNotFound(t *testing.T) {
	_, ts := openBuildsTestServer(t)

	resp, err := http.Get(ts.URL + "/apps/00000000-0000-0000-0000-000000000000/builds")
	if err != nil {
		t.Fatalf("GET /apps/{id}/builds: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	assertErrorMessage(t, resp, "app not found")
}

func TestGetBuild_NotFound(t *testing.T) {
	_, ts := openBuildsTestServer(t)
	name := "builds-missing-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)

	resp, err := http.Get(ts.URL + "/apps/" + app.ID + "/builds/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("GET /apps/{id}/builds/{build_id}: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	assertErrorMessage(t, resp, "build not found")
}

func TestGetBuild_WrongApp(t *testing.T) {
	st, ts := openBuildsTestServer(t)
	a := createApp(t, ts, "builds-owner-a-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	b := createApp(t, ts, "builds-owner-b-"+strconv.FormatInt(time.Now().UnixNano(), 10))

	created, err := st.CreateBuild(context.Background(), a.ID)
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}

	resp, err := http.Get(ts.URL + "/apps/" + b.ID + "/builds/" + created.ID)
	if err != nil {
		t.Fatalf("GET /apps/{id}/builds/{build_id}: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	assertErrorMessage(t, resp, "build not found")
}

func TestBuildsListAndGet(t *testing.T) {
	st, ts := openBuildsTestServer(t)
	name := "builds-crud-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)

	first, err := st.CreateBuild(context.Background(), app.ID)
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	second, err := st.CreateBuild(context.Background(), app.ID)
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}

	builds := listBuilds(t, ts, app.ID)
	if len(builds) < 2 {
		t.Fatalf("list builds len = %d, want at least 2", len(builds))
	}
	if builds[0].ID != second.ID {
		t.Fatalf("newest build = %q, want %q", builds[0].ID, second.ID)
	}
	if builds[1].ID != first.ID {
		t.Fatalf("older build = %q, want %q", builds[1].ID, first.ID)
	}
	if builds[0].Status != string(build.StatusPending) {
		t.Fatalf("status = %q, want %q", builds[0].Status, build.StatusPending)
	}

	got := getBuild(t, ts, app.ID, first.ID)
	if got.ID != first.ID || got.AppID != app.ID {
		t.Fatalf("get build = %+v, want id=%s app_id=%s", got, first.ID, app.ID)
	}
	if got.Status != string(build.StatusPending) {
		t.Fatalf("status = %q, want %q", got.Status, build.StatusPending)
	}
	if got.Image != "" {
		t.Fatalf("image = %q, want empty before push", got.Image)
	}
}

func TestBuildImage_RoundTrip(t *testing.T) {
	st, ts := openBuildsTestServer(t)
	name := "builds-image-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)

	created, err := st.CreateBuild(context.Background(), app.ID)
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}

	image := "localhost:5000/atlas/" + app.ID + ":" + created.ID
	if _, err := st.UpdateBuildImage(context.Background(), created.ID, image); err != nil {
		t.Fatalf("UpdateBuildImage: %v", err)
	}

	got := getBuild(t, ts, app.ID, created.ID)
	if got.Image != image {
		t.Fatalf("get image = %q, want %q", got.Image, image)
	}

	builds := listBuilds(t, ts, app.ID)
	if len(builds) != 1 {
		t.Fatalf("list builds len = %d, want 1", len(builds))
	}
	if builds[0].Image != image {
		t.Fatalf("list image = %q, want %q", builds[0].Image, image)
	}
}

func openBuildsTestServer(t *testing.T) (*store.Store, *httptest.Server) {
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

	return st, testBuildsServer(t, st)
}

func listBuilds(t *testing.T, ts *httptest.Server, appID string) []buildResponse {
	t.Helper()

	resp, err := http.Get(ts.URL + "/apps/" + appID + "/builds")
	if err != nil {
		t.Fatalf("GET /apps/%s/builds: %v", appID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var builds []buildResponse
	decodeJSON(t, resp.Body, &builds)
	return builds
}

func getBuild(t *testing.T, ts *httptest.Server, appID, buildID string) buildResponse {
	t.Helper()

	resp, err := http.Get(ts.URL + "/apps/" + appID + "/builds/" + buildID)
	if err != nil {
		t.Fatalf("GET /apps/%s/builds/%s: %v", appID, buildID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got buildResponse
	decodeJSON(t, resp.Body, &got)
	return got
}
