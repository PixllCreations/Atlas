package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pixll/atlas/store"
)

func testReposServer(t *testing.T, st *store.Store) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	RegisterApps(mux, st, nil, "")
	RegisterRepos(mux, st, nil)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestLinkRepo_EmptyURL(t *testing.T) {
	_, ts := openReposTestServer(t)
	name := "repo-empty-url-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)

	resp := putRepo(t, ts, app.ID, `{"url":""}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	assertErrorMessage(t, resp, "url or github_full_name is required")
}

func TestLinkRepo_UnsupportedProvider(t *testing.T) {
	_, ts := openReposTestServer(t)
	name := "repo-bad-provider-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)

	resp := putRepo(t, ts, app.ID, `{"url":"https://gitlab.com/user/repo","provider":"gitlab"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	assertErrorMessage(t, resp, "unsupported provider")
}

func TestReposLinkGetUnlink(t *testing.T) {
	_, ts := openReposTestServer(t)
	name := "repo-crud-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)

	linked := linkRepo(t, ts, app.ID, `{"url":"https://github.com/user/portfolio"}`)
	if linked.URL != "https://github.com/user/portfolio" {
		t.Fatalf("linked url = %q, want portfolio repo", linked.URL)
	}
	if linked.Provider != "github" {
		t.Fatalf("linked provider = %q, want github", linked.Provider)
	}
	if linked.Branch != "main" {
		t.Fatalf("linked branch = %q, want main", linked.Branch)
	}

	got := getRepo(t, ts, app.ID)
	if got.URL != linked.URL || got.Provider != linked.Provider || got.Branch != linked.Branch {
		t.Fatalf("get repo = %+v, want %+v", got, linked)
	}

	unlinkRepo(t, ts, app.ID)
	assertRepoNotFound(t, ts, app.ID)
}

func TestGetRepo_AppNotFound(t *testing.T) {
	_, ts := openReposTestServer(t)

	resp, err := http.Get(ts.URL + "/apps/00000000-0000-0000-0000-000000000000/repo")
	if err != nil {
		t.Fatalf("GET /apps/{id}/repo: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	assertErrorMessage(t, resp, "app not found")
}

func openReposTestServer(t *testing.T) (*store.Store, *httptest.Server) {
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

	return st, testReposServer(t, st)
}

func putRepo(t *testing.T, ts *httptest.Server, appID, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/apps/"+appID+"/repo", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("NewRequest PUT /apps/%s/repo: %v", appID, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /apps/%s/repo: %v", appID, err)
	}
	return resp
}

func linkRepo(t *testing.T, ts *httptest.Server, appID, body string) repoResponse {
	t.Helper()

	resp := putRepo(t, ts, appID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var linked repoResponse
	decodeJSON(t, resp.Body, &linked)
	return linked
}

func getRepo(t *testing.T, ts *httptest.Server, appID string) repoResponse {
	t.Helper()

	resp, err := http.Get(ts.URL + "/apps/" + appID + "/repo")
	if err != nil {
		t.Fatalf("GET /apps/%s/repo: %v", appID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var repo repoResponse
	decodeJSON(t, resp.Body, &repo)
	return repo
}

func unlinkRepo(t *testing.T, ts *httptest.Server, appID string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/apps/"+appID+"/repo", nil)
	if err != nil {
		t.Fatalf("NewRequest DELETE /apps/%s/repo: %v", appID, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /apps/%s/repo: %v", appID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func assertRepoNotFound(t *testing.T, ts *httptest.Server, appID string) {
	t.Helper()

	resp, err := http.Get(ts.URL + "/apps/" + appID + "/repo")
	if err != nil {
		t.Fatalf("GET /apps/%s/repo: %v", appID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	assertErrorMessage(t, resp, "repo not found")
}

func ensureMigrations(t *testing.T, ctx context.Context, dsn string) {
	t.Helper()

	matches, err := filepath.Glob("../store/migrations/*.sql")
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	sort.Strings(matches)

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect for migration: %v", err)
	}
	defer conn.Close(ctx)

	for _, path := range matches {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply migration %s: %v", path, err)
		}
	}
}
