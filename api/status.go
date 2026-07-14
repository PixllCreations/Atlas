package api

import (
	"log"
	"net/http"

	"github.com/pixll/atlas/store"
)

type InstallationResponse struct {
	ID           int64  `json:"id"`
	AccountLogin string `json:"account_login"`
	AccountType  string `json:"account_type"`
}

// StatusConfig exposes runtime hints for the console status page.
type StatusConfig struct {
	Port                string
	IngressDomain       string
	RegistryURL         string
	Namespace           string
	KubernetesOK        bool
	WebhookSecret       bool
	WebhookPublicURL    string
	GitHubAppConfigured bool
	GitHubAppSlug       string
}

type statusResponse struct {
	OK                  bool                   `json:"ok"`
	Port                string                 `json:"port"`
	IngressDomain       string                 `json:"ingress_domain"`
	RegistrySet         bool                   `json:"registry_set"`
	Namespace           string                 `json:"namespace"`
	Kubernetes          bool                   `json:"kubernetes"`
	WebhookConfig       bool                   `json:"webhook_configured"`
	WebhookPublicURL    string                 `json:"webhook_public_url"`
	GitHubAppConfigured bool                   `json:"github_app_configured"`
	GitHubAppSlug       string                 `json:"github_app_slug,omitempty"`
	GitHubInstallations []InstallationResponse `json:"github_installations,omitempty"`
}

func RegisterStatus(mux *http.ServeMux, cfg StatusConfig, st *store.Store) {
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		ns := cfg.Namespace
		if ns == "" {
			ns = "default"
		}

		var installations []InstallationResponse
		if st != nil {
			insts, err := st.ListInstallations(r.Context())
			if err != nil {
				log.Printf("list github installations for status: %v", err)
			} else {
				installations = make([]InstallationResponse, 0, len(insts))
				for _, inst := range insts {
					installations = append(installations, InstallationResponse{
						ID:           inst.ID,
						AccountLogin: inst.AccountLogin,
						AccountType:  inst.AccountType,
					})
				}
			}
		}

		writeJSON(w, http.StatusOK, statusResponse{
			OK:                  true,
			Port:                cfg.Port,
			IngressDomain:       cfg.IngressDomain,
			RegistrySet:         cfg.RegistryURL != "",
			Namespace:           ns,
			Kubernetes:          cfg.KubernetesOK,
			WebhookConfig:       cfg.WebhookSecret,
			WebhookPublicURL:    cfg.WebhookPublicURL,
			GitHubAppConfigured: cfg.GitHubAppConfigured,
			GitHubAppSlug:       cfg.GitHubAppSlug,
			GitHubInstallations: installations,
		})
	})
}
