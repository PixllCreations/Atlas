package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pixll/atlas/store"
)

func testAppsServer(t *testing.T, st *store.Store) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	RegisterApps(mux, st, nil, "")
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestCreateApp_InvalidJSON(t *testing.T) {
	ts := testAppsServer(t, nil)

	resp, err := http.Post(ts.URL+"/apps", "application/json", bytes.NewBufferString("{"))
	if err != nil {
		t.Fatalf("POST /apps: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	assertErrorMessage(t, resp, "invalid JSON body")
}

func TestCreateApp_EmptyName(t *testing.T) {
	ts := testAppsServer(t, nil)

	resp, err := http.Post(ts.URL+"/apps", "application/json", bytes.NewBufferString(`{"name":""}`))
	if err != nil {
		t.Fatalf("POST /apps: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	assertErrorMessage(t, resp, "name is required")
}

func TestAppsCRUD(t *testing.T) {
	dsn := os.Getenv("ATLAS_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://atlas:atlas@localhost:5432/atlas?sslmode=disable"
	}

	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer st.Close()
	ensureAppsTable(t, ctx, dsn)

	ts := testAppsServer(t, st)
	name := "test-app-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	created := createApp(t, ts, name)
	if created.Name != name {
		t.Fatalf("created name = %q, want %q", created.Name, name)
	}

	got := getApp(t, ts, created.ID)
	if got.ID != created.ID || got.Name != name {
		t.Fatalf("get app = %+v, want id=%s name=%s", got, created.ID, name)
	}

	apps := listApps(t, ts)
	if !containsApp(apps, created.ID) {
		t.Fatalf("list apps missing %s: %+v", created.ID, apps)
	}

	deleteApp(t, ts, created.ID)
	assertAppNotFound(t, ts, created.ID)
}

func createApp(t *testing.T, ts *httptest.Server, name string) appResponse {
	t.Helper()

	body := `{"name":"` + name + `"}`
	resp, err := http.Post(ts.URL+"/apps", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /apps: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var created appResponse
	decodeJSON(t, resp.Body, &created)

	t.Cleanup(func() {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/apps/"+created.ID, nil)
		if err != nil {
			return
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		_ = r.Body.Close()
	})

	return created
}

func getApp(t *testing.T, ts *httptest.Server, id string) appResponse {
	t.Helper()

	resp, err := http.Get(ts.URL + "/apps/" + id)
	if err != nil {
		t.Fatalf("GET /apps/%s: %v", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got appResponse
	decodeJSON(t, resp.Body, &got)
	return got
}

func listApps(t *testing.T, ts *httptest.Server) []appResponse {
	t.Helper()

	resp, err := http.Get(ts.URL + "/apps")
	if err != nil {
		t.Fatalf("GET /apps: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var apps []appResponse
	decodeJSON(t, resp.Body, &apps)
	return apps
}

func deleteApp(t *testing.T, ts *httptest.Server, id string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/apps/"+id, nil)
	if err != nil {
		t.Fatalf("NewRequest DELETE /apps/%s: %v", id, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /apps/%s: %v", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func assertAppNotFound(t *testing.T, ts *httptest.Server, id string) {
	t.Helper()

	resp, err := http.Get(ts.URL + "/apps/" + id)
	if err != nil {
		t.Fatalf("GET /apps/%s: %v", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	assertErrorMessage(t, resp, "app not found")
}

func assertErrorMessage(t *testing.T, resp *http.Response, want string) {
	t.Helper()

	var body map[string]string
	decodeJSON(t, resp.Body, &body)
	if body["error"] != want {
		t.Fatalf("error = %q, want %q", body["error"], want)
	}
}

func decodeJSON(t *testing.T, r io.Reader, v any) {
	t.Helper()

	if err := json.NewDecoder(r).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

func containsApp(apps []appResponse, id string) bool {
	for _, a := range apps {
		if a.ID == id {
			return true
		}
	}
	return false
}

func ensureAppsTable(t *testing.T, ctx context.Context, dsn string) {
	t.Helper()

	for _, name := range []string{"001_apps.sql", "006_app_port.sql"} {
		sql, err := os.ReadFile("../store/migrations/" + name)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}

		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatalf("connect for migration: %v", err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			conn.Close(ctx)
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			t.Fatalf("apply migration %s: %v", name, err)
		}
		conn.Close(ctx)
	}
}
