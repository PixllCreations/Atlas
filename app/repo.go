package app

// Provider identifies a git hosting service.
type Provider string

const (
	ProviderGitHub Provider = "github"
)

// Repo is the git source Atlas deploys for an App.
type Repo struct {
	URL            string
	Provider       Provider
	Branch         string
	GitHubRepoID   int64
	GitHubFullName string
	InstallationID int64
}
