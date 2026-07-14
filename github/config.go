package github

import (
	"os"
	"strconv"
	"strings"
)

// Config holds GitHub App credentials for Atlas.
type Config struct {
	AppID        int64
	AppSlug      string
	PrivateKey   []byte
	ClientID     string
	ClientSecret string
}

// Enabled reports whether GitHub App integration is configured.
func (c Config) Enabled() bool {
	return c.AppID > 0 && len(c.PrivateKey) > 0 && c.AppSlug != ""
}

// LoadConfig reads GitHub App settings from the environment.
func LoadConfig() (Config, error) {
	appID, _ := strconv.ParseInt(strings.TrimSpace(os.Getenv("ATLAS_GITHUB_APP_ID")), 10, 64)
	slug := strings.TrimSpace(os.Getenv("ATLAS_GITHUB_APP_SLUG"))

	key, err := loadPrivateKey(
		os.Getenv("ATLAS_GITHUB_APP_PRIVATE_KEY"),
		os.Getenv("ATLAS_GITHUB_APP_PRIVATE_KEY_PATH"),
	)
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppID:        appID,
		AppSlug:      slug,
		PrivateKey:   key,
		ClientID:     strings.TrimSpace(os.Getenv("ATLAS_GITHUB_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("ATLAS_GITHUB_CLIENT_SECRET")),
	}, nil
}

func loadPrivateKey(inline, path string) ([]byte, error) {
	inline = strings.TrimSpace(inline)
	if inline != "" {
		return []byte(inline), nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}
