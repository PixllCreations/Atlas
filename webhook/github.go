package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const GitHubSignatureHeader = "X-Hub-Signature-256"

var ErrInvalidSignature = errors.New("invalid webhook signature")

// GitHubPushEvent is the subset of a GitHub push payload Atlas cares about.
type GitHubPushEvent struct {
	Ref        string `json:"ref"`
	Repository struct {
		HTMLURL string `json:"html_url"`
	} `json:"repository"`
}

// VerifyGitHubSignature validates the HMAC SHA-256 signature from GitHub.
func VerifyGitHubSignature(secret string, body []byte, signature string) error {
	if secret == "" {
		return errors.New("webhook secret not configured")
	}
	if !strings.HasPrefix(signature, "sha256=") {
		return ErrInvalidSignature
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return ErrInvalidSignature
	}
	return nil
}

// ParseGitHubPush decodes a GitHub push webhook payload.
func ParseGitHubPush(body []byte) (GitHubPushEvent, error) {
	var event GitHubPushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return GitHubPushEvent{}, fmt.Errorf("parse github push: %w", err)
	}
	return event, nil
}

// BranchFromRef extracts the branch name from refs/heads/main.
func BranchFromRef(ref string) (string, error) {
	const prefix = "refs/heads/"
	if !strings.HasPrefix(ref, prefix) {
		return "", fmt.Errorf("unsupported ref: %s", ref)
	}
	return strings.TrimPrefix(ref, prefix), nil
}
