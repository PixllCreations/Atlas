package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pixll/atlas/build"
)

func TestHealthz(t *testing.T) {
	srv := New(Options{
		Addr:         ":8080",
		WorkerConfig: build.WorkerConfig{},
	})
	ts := httptest.NewServer(srv.mux)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want %q", body, "ok")
	}
}

func TestStatus(t *testing.T) {
	srv := New(Options{
		Addr: ":8080",
		Status: StatusConfig{
			Port:             "8080",
			IngressDomain:    "edwardscott.dev",
			RegistryURL:      "localhost:5000",
			Namespace:        "default",
			KubernetesOK:     true,
			WebhookSecret:    true,
			WebhookPublicURL: "https://hooks.edwardscott.dev/webhooks/github",
		},
	})
	ts := httptest.NewServer(srv.mux)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/status")
	if err != nil {
		t.Fatalf("GET /status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got statusResponse
	decodeJSON(t, resp.Body, &got)
	if !got.OK || !got.RegistrySet || !got.Kubernetes || got.IngressDomain != "edwardscott.dev" {
		t.Fatalf("status response = %+v", got)
	}
	if got.WebhookPublicURL != "https://hooks.edwardscott.dev/webhooks/github" {
		t.Fatalf("webhook_public_url = %q", got.WebhookPublicURL)
	}
}
