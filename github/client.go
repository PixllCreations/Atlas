package github

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const apiBase = "https://api.github.com"

// Client talks to the GitHub REST API as a GitHub App.
type Client struct {
	cfg        Config
	privateKey *rsa.PrivateKey
	http       *http.Client

	mu     sync.Mutex
	tokens map[int64]cachedToken
}

type cachedToken struct {
	value     string
	expiresAt time.Time
}

// Installation describes a GitHub App installation.
type Installation struct {
	ID           int64  `json:"id"`
	AccountLogin string `json:"account_login"`
	AccountType  string `json:"account_type"`
}

type apiInstallation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
}

// Repository is a GitHub repo visible to an installation.
type Repository struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
}

// NewClient builds a GitHub App API client.
func NewClient(cfg Config) (*Client, error) {
	if !cfg.Enabled() {
		return nil, fmt.Errorf("github app not configured")
	}
	key, err := parsePrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &Client{
		cfg:        cfg,
		privateKey: key,
		http:       &http.Client{Timeout: 30 * time.Second},
		tokens:     make(map[int64]cachedToken),
	}, nil
}

// InstallURL returns the URL where a user installs the GitHub App.
func (c *Client) InstallURL(state string) string {
	u := fmt.Sprintf("https://github.com/apps/%s/installations/new", c.cfg.AppSlug)
	if state != "" {
		u += "?state=" + url.QueryEscape(state)
	}
	return u
}

// GetInstallation fetches installation metadata from GitHub.
func (c *Client) GetInstallation(ctx context.Context, installationID int64) (Installation, error) {
	var raw apiInstallation
	if err := c.appRequest(ctx, http.MethodGet, fmt.Sprintf("/app/installations/%d", installationID), nil, &raw); err != nil {
		return Installation{}, err
	}
	return Installation{
		ID:           raw.ID,
		AccountLogin: raw.Account.Login,
		AccountType:  raw.Account.Type,
	}, nil
}

// ListAppInstallations lists all installations of this GitHub App.
func (c *Client) ListAppInstallations(ctx context.Context) ([]Installation, error) {
	var raw []apiInstallation
	if err := c.appRequest(ctx, http.MethodGet, "/app/installations?per_page=100", nil, &raw); err != nil {
		return nil, err
	}
	out := make([]Installation, 0, len(raw))
	for _, item := range raw {
		out = append(out, Installation{
			ID:           item.ID,
			AccountLogin: item.Account.Login,
			AccountType:  item.Account.Type,
		})
	}
	return out, nil
}

// ListInstallationRepos returns repositories granted to an installation.
func (c *Client) ListInstallationRepos(ctx context.Context, installationID int64) ([]Repository, error) {
	var page struct {
		Repositories []Repository `json:"repositories"`
	}
	if err := c.installationRequest(ctx, installationID, http.MethodGet, "/installation/repositories?per_page=100", nil, &page); err != nil {
		return nil, err
	}
	return page.Repositories, nil
}

// GetRepository looks up a repository by full name within an installation.
func (c *Client) GetRepository(ctx context.Context, installationID int64, fullName string) (Repository, error) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return Repository{}, fmt.Errorf("repository name is required")
	}
	repos, err := c.ListInstallationRepos(ctx, installationID)
	if err != nil {
		return Repository{}, err
	}
	for _, repo := range repos {
		if strings.EqualFold(repo.FullName, fullName) {
			return repo, nil
		}
	}
	return Repository{}, fmt.Errorf("repository %q not accessible to installation", fullName)
}

// InstallationToken returns a short-lived token for an installation.
func (c *Client) InstallationToken(ctx context.Context, installationID int64) (string, error) {
	c.mu.Lock()
	if cached, ok := c.tokens[installationID]; ok && time.Now().Before(cached.expiresAt.Add(-time.Minute)) {
		c.mu.Unlock()
		return cached.value, nil
	}
	c.mu.Unlock()

	var resp struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := c.appRequest(ctx, http.MethodPost, fmt.Sprintf("/app/installations/%d/access_tokens", installationID), nil, &resp); err != nil {
		return "", err
	}
	if resp.Token == "" {
		return "", fmt.Errorf("empty installation token")
	}

	c.mu.Lock()
	c.tokens[installationID] = cachedToken{value: resp.Token, expiresAt: resp.ExpiresAt}
	c.mu.Unlock()
	return resp.Token, nil
}

// CloneURL returns an authenticated HTTPS clone URL for a repository.
func (c *Client) CloneURL(ctx context.Context, installationID int64, fullName string) (string, error) {
	token, err := c.InstallationToken(ctx, installationID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, fullName), nil
}

func (c *Client) appRequest(ctx context.Context, method, path string, body io.Reader, out any) error {
	jwt, err := signAppJWT(c.cfg.AppID, c.privateKey, time.Now())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return c.do(req, out)
}

func (c *Client) installationRequest(ctx context.Context, installationID int64, method, path string, body io.Reader, out any) error {
	token, err := c.InstallationToken(ctx, installationID)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read github response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("github api %s %s: %s", req.Method, req.URL.Path, strings.TrimSpace(string(data)))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	return nil
}
