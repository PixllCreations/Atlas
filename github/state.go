package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidState = errors.New("invalid install state")

type installState struct {
	Return string `json:"return"`
	Exp    int64  `json:"exp"`
}

// SignInstallState returns a signed state token for the GitHub App install redirect.
func SignInstallState(secret, returnPath string, now time.Time) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", fmt.Errorf("state secret required")
	}
	if returnPath == "" {
		returnPath = "/"
	}
	payload, err := json.Marshal(installState{
		Return: returnPath,
		Exp:    now.Add(15 * time.Minute).Unix(),
	})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encoded))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + sig, nil
}

// VerifyInstallState validates and decodes a signed install state token.
func VerifyInstallState(secret, token string, now time.Time) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", ErrInvalidState
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", ErrInvalidState
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return "", ErrInvalidState
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", ErrInvalidState
	}
	var state installState
	if err := json.Unmarshal(raw, &state); err != nil {
		return "", ErrInvalidState
	}
	if state.Exp < now.Unix() {
		return "", ErrInvalidState
	}
	if state.Return == "" {
		return "/", nil
	}
	if !strings.HasPrefix(state.Return, "/") || strings.HasPrefix(state.Return, "//") {
		return "", ErrInvalidState
	}
	return state.Return, nil
}
