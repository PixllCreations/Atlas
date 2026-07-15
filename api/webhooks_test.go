package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pixll/atlas/app"
	"github.com/pixll/atlas/build"
	"github.com/pixll/atlas/store"
	"github.com/pixll/atlas/webhook"
)

const testWebhookSecret = "test-webhook-secret"

func testWebhooksServer(t *testing.T, st *store.Store, secret string, worker *build.Worker) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	RegisterApps(mux, st, nil, "")
	RegisterRepos(mux, st, nil)
	RegisterWebhooks(mux, st, secret, worker)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func signGitHubPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func githubPushBody(repoURL, ref string) []byte {
	return []byte(fmt.Sprintf(
		`{"ref":%q,"repository":{"html_url":%q}}`,
		ref, repoURL,
	))
}

func githubPushBodyWithID(repoID int64, repoURL, ref string) []byte {
	return []byte(fmt.Sprintf(
		`{"ref":%q,"repository":{"id":%d,"html_url":%q,"full_name":"user/repo"}}`,
		ref, repoID, repoURL,
	))
}

func postGitHubWebhook(t *testing.T, ts *httptest.Server, secret, eventType string, body []byte, signature string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/webhooks/github", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest POST /webhooks/github: %v", err)
	}
	req.Header.Set("X-GitHub-Event", eventType)
	req.Header.Set("Content-Type", "application/json")
	if signature != "" {
		req.Header.Set(webhook.GitHubSignatureHeader, signature)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /webhooks/github: %v", err)
	}
	return resp
}

func TestWebhook_NotPushEvent(t *testing.T) {
	ts := testWebhooksServer(t, nil, testWebhookSecret, nil)

	resp := postGitHubWebhook(t, ts, testWebhookSecret, "ping", []byte(`{}`), signGitHubPayload(testWebhookSecret, []byte(`{}`)))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestWebhook_InvalidSignature(t *testing.T) {
	ts := testWebhooksServer(t, nil, testWebhookSecret, nil)
	body := githubPushBody("https://github.com/user/repo", "refs/heads/main")

	resp := postGitHubWebhook(t, ts, testWebhookSecret, "push", body, "sha256=deadbeef")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	assertErrorMessage(t, resp, "invalid signature")
}

func TestWebhook_SecretNotConfigured(t *testing.T) {
	ts := testWebhooksServer(t, nil, "", nil)
	body := githubPushBody("https://github.com/user/repo", "refs/heads/main")

	resp := postGitHubWebhook(t, ts, "", "push", body, signGitHubPayload("any-secret", body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	assertErrorMessage(t, resp, "webhook not configured")
}

func TestWebhook_InvalidPayload(t *testing.T) {
	ts := testWebhooksServer(t, nil, testWebhookSecret, nil)
	body := []byte(`{not json`)

	resp := postGitHubWebhook(t, ts, testWebhookSecret, "push", body, signGitHubPayload(testWebhookSecret, body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	assertErrorMessage(t, resp, "invalid payload")
}

func TestWebhook_TagPushIgnored(t *testing.T) {
	ts := testWebhooksServer(t, nil, testWebhookSecret, nil)
	body := githubPushBody("https://github.com/user/repo", "refs/tags/v1.0.0")

	resp := postGitHubWebhook(t, ts, testWebhookSecret, "push", body, signGitHubPayload(testWebhookSecret, body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestWebhook_AcceptedPushByRepoID(t *testing.T) {
	st, ts := openWebhooksTestServer(t)
	name := "webhook-repo-id-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	created := createApp(t, ts, name)
	repoURL := "https://github.com/user/" + name
	const repoID int64 = 42424242

	if err := st.UpsertInstallation(context.Background(), store.GitHubInstallation{
		ID: 999000001, AccountLogin: "atlas-test-user", AccountType: "User",
	}); err != nil {
		t.Fatalf("UpsertInstallation: %v", err)
	}
	t.Cleanup(func() {
		_ = st.UnlinkReposByGitHubIDs(context.Background(), []int64{repoID})
		_ = st.DeleteInstallation(context.Background(), 999000001)
	})

	_, err := st.LinkRepo(context.Background(), created.ID, app.Repo{
		URL:            repoURL,
		Provider:       app.ProviderGitHub,
		Branch:         "main",
		GitHubRepoID:   repoID,
		GitHubFullName: "user/" + name,
		InstallationID: 999000001,
	})
	if err != nil {
		t.Fatalf("LinkRepo with github id: %v", err)
	}

	body := githubPushBodyWithID(repoID, "https://github.com/user/renamed", "refs/heads/main")
	resp := postGitHubWebhook(t, ts, testWebhookSecret, "push", body, signGitHubPayload(testWebhookSecret, body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}

func TestWebhook_AcceptedPush(t *testing.T) {
	st, ts := openWebhooksTestServer(t)
	name := "webhook-accept-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)
	repoURL := "https://github.com/user/" + name
	linkRepo(t, ts, app.ID, fmt.Sprintf(`{"url":%q}`, repoURL))

	body := githubPushBody(repoURL, "refs/heads/main")
	resp := postGitHubWebhook(t, ts, testWebhookSecret, "push", body, signGitHubPayload(testWebhookSecret, body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	var got map[string]string
	decodeJSON(t, resp.Body, &got)
	if got["status"] != "accepted" {
		t.Fatalf("status = %q, want accepted", got["status"])
	}
	if got["app_id"] != app.ID {
		t.Fatalf("app_id = %q, want %q", got["app_id"], app.ID)
	}
	if got["build_id"] == "" {
		t.Fatal("build_id missing from response")
	}

	b := waitForBuildStatus(t, st, got["build_id"], build.StatusSucceeded)
	if b.AppID != app.ID {
		t.Fatalf("build app_id = %q, want %q", b.AppID, app.ID)
	}
}

func TestWebhook_UnlinkedRepo(t *testing.T) {
	_, ts := openWebhooksTestServer(t)
	body := githubPushBody("https://github.com/user/unlinked", "refs/heads/main")

	resp := postGitHubWebhook(t, ts, testWebhookSecret, "push", body, signGitHubPayload(testWebhookSecret, body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestWebhook_WrongBranch(t *testing.T) {
	_, ts := openWebhooksTestServer(t)
	name := "webhook-branch-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	app := createApp(t, ts, name)
	repoURL := "https://github.com/user/" + name
	linkRepo(t, ts, app.ID, fmt.Sprintf(`{"url":%q,"branch":"main"}`, repoURL))

	body := githubPushBody(repoURL, "refs/heads/develop")
	resp := postGitHubWebhook(t, ts, testWebhookSecret, "push", body, signGitHubPayload(testWebhookSecret, body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func openWebhooksTestServer(t *testing.T) (*store.Store, *httptest.Server) {
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

	return st, testWebhooksServer(t, st, testWebhookSecret, build.NewWorkerWithHooks(st, build.WorkerConfig{}, nil, nil,
		func(_ context.Context, _, _, dest string) error {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dest, "atlas.yaml"), []byte("version: 1\napp:\n  port: 8080\n"), 0o644)
		},
		func(context.Context, string, string) error { return nil },
		func(context.Context, string, string) error { return nil },
	))
}

func waitForBuildStatus(t *testing.T, st *store.Store, buildID string, want build.Status) build.Build {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, err := st.GetBuild(context.Background(), buildID)
		if err != nil {
			t.Fatalf("GetBuild(%q): %v", buildID, err)
		}
		if b.Status == want {
			return b
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("build %s did not reach status %q", buildID, want)
	return build.Build{}
}
