package api

import (
	"net/url"
	"strconv"
	"strings"
)

func publicBaseURL(webhookPublicURL string) string {
	webhookPublicURL = strings.TrimSpace(webhookPublicURL)
	if webhookPublicURL == "" {
		return "http://localhost:8080"
	}
	u, err := url.Parse(webhookPublicURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "http://localhost:8080"
	}
	return u.Scheme + "://" + u.Host
}

func parseInstallationID(raw string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
}
