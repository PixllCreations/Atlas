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
	"strconv"
	"testing"
	"time"

	"github.com/pixll/atlas/store"
	"github.com/pixll/atlas/webhook"
)

const testWebhookSecret = "test-webhook-secret"

func testWebhooksServer(t *testing.T, st *store.Store, secret string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	RegisterApps(mux, st)
	RegisterRepos(mux, st)
	RegisterWebhooks(mux, st, secret)
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
	ts := testWebhooksServer(t, nil, testWebhookSecret)

	resp := postGitHubWebhook(t, ts, testWebhookSecret, "ping", []byte(`{}`), signGitHubPayload(testWebhookSecret, []byte(`{}`)))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestWebhook_InvalidSignature(t *testing.T) {
	ts := testWebhooksServer(t, nil, testWebhookSecret)
	body := githubPushBody("https://github.com/user/repo", "refs/heads/main")

	resp := postGitHubWebhook(t, ts, testWebhookSecret, "push", body, "sha256=deadbeef")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	assertErrorMessage(t, resp, "invalid signature")
}

func TestWebhook_SecretNotConfigured(t *testing.T) {
	ts := testWebhooksServer(t, nil, "")
	body := githubPushBody("https://github.com/user/repo", "refs/heads/main")

	resp := postGitHubWebhook(t, ts, "", "push", body, signGitHubPayload("any-secret", body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	assertErrorMessage(t, resp, "webhook not configured")
}

func TestWebhook_InvalidPayload(t *testing.T) {
	ts := testWebhooksServer(t, nil, testWebhookSecret)
	body := []byte(`{not json`)

	resp := postGitHubWebhook(t, ts, testWebhookSecret, "push", body, signGitHubPayload(testWebhookSecret, body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	assertErrorMessage(t, resp, "invalid payload")
}

func TestWebhook_TagPushIgnored(t *testing.T) {
	ts := testWebhooksServer(t, nil, testWebhookSecret)
	body := githubPushBody("https://github.com/user/repo", "refs/tags/v1.0.0")

	resp := postGitHubWebhook(t, ts, testWebhookSecret, "push", body, signGitHubPayload(testWebhookSecret, body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestWebhook_AcceptedPush(t *testing.T) {
	_, ts := openWebhooksTestServer(t)
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

	return st, testWebhooksServer(t, st, testWebhookSecret)
}
