package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func signGitHubPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyGitHubSignature_Valid(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"ref":"refs/heads/main"}`)

	err := VerifyGitHubSignature(secret, body, signGitHubPayload(secret, body))
	if err != nil {
		t.Fatalf("VerifyGitHubSignature() = %v, want nil", err)
	}
}

func TestVerifyGitHubSignature_WrongSecret(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)

	err := VerifyGitHubSignature("correct-secret", body, signGitHubPayload("wrong-secret", body))
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("VerifyGitHubSignature() = %v, want %v", err, ErrInvalidSignature)
	}
}

func TestVerifyGitHubSignature_TamperedBody(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"ref":"refs/heads/main"}`)
	sig := signGitHubPayload(secret, body)

	err := VerifyGitHubSignature(secret, []byte(`{"ref":"refs/heads/evil"}`), sig)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("VerifyGitHubSignature() = %v, want %v", err, ErrInvalidSignature)
	}
}

func TestVerifyGitHubSignature_MissingPrefix(t *testing.T) {
	err := VerifyGitHubSignature("secret", []byte("body"), "not-a-github-signature")
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("VerifyGitHubSignature() = %v, want %v", err, ErrInvalidSignature)
	}
}

func TestVerifyGitHubSignature_EmptySecret(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main"}`)

	err := VerifyGitHubSignature("", body, signGitHubPayload("secret", body))
	if err == nil {
		t.Fatal("VerifyGitHubSignature() = nil, want error")
	}
	if err.Error() != "webhook secret not configured" {
		t.Fatalf("VerifyGitHubSignature() = %q, want %q", err.Error(), "webhook secret not configured")
	}
}

func TestParseGitHubPush_Valid(t *testing.T) {
	body := []byte(`{
		"ref": "refs/heads/main",
		"repository": {"html_url": "https://github.com/pixll/portfolio"}
	}`)

	event, err := ParseGitHubPush(body)
	if err != nil {
		t.Fatalf("ParseGitHubPush() = %v, want nil", err)
	}
	if event.Ref != "refs/heads/main" {
		t.Fatalf("Ref = %q, want %q", event.Ref, "refs/heads/main")
	}
	if event.Repository.HTMLURL != "https://github.com/pixll/portfolio" {
		t.Fatalf("Repository.HTMLURL = %q, want %q", event.Repository.HTMLURL, "https://github.com/pixll/portfolio")
	}
}

func TestParseGitHubPush_InvalidJSON(t *testing.T) {
	_, err := ParseGitHubPush([]byte(`{not json`))
	if err == nil {
		t.Fatal("ParseGitHubPush() = nil, want error")
	}
	if !strings.Contains(err.Error(), "parse github push") {
		t.Fatalf("ParseGitHubPush() = %q, want wrapped parse error", err.Error())
	}
}

func TestBranchFromRef_Main(t *testing.T) {
	branch, err := BranchFromRef("refs/heads/main")
	if err != nil {
		t.Fatalf("BranchFromRef() = %v, want nil", err)
	}
	if branch != "main" {
		t.Fatalf("branch = %q, want %q", branch, "main")
	}
}

func TestBranchFromRef_FeatureBranch(t *testing.T) {
	branch, err := BranchFromRef("refs/heads/feature/auth")
	if err != nil {
		t.Fatalf("BranchFromRef() = %v, want nil", err)
	}
	if branch != "feature/auth" {
		t.Fatalf("branch = %q, want %q", branch, "feature/auth")
	}
}

func TestBranchFromRef_Unsupported(t *testing.T) {
	_, err := BranchFromRef("refs/tags/v1.0.0")
	if err == nil {
		t.Fatal("BranchFromRef() = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported ref") {
		t.Fatalf("BranchFromRef() = %q, want unsupported ref error", err.Error())
	}
}
