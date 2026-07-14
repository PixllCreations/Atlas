package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pixll/atlas/app"
)

var ErrRepoNotFound = errors.New("app repo not found")

func (s *Store) LinkRepo(ctx context.Context, appID string, repo app.Repo) (app.Repo, error) {
	const q = `
		INSERT INTO app_repos (
			app_id, url, provider, branch,
			github_repo_id, github_full_name, installation_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (app_id) DO UPDATE
		SET url = EXCLUDED.url,
			provider = EXCLUDED.provider,
			branch = EXCLUDED.branch,
			github_repo_id = EXCLUDED.github_repo_id,
			github_full_name = EXCLUDED.github_full_name,
			installation_id = EXCLUDED.installation_id,
			updated_at = now()
		RETURNING url, provider, branch, github_repo_id, github_full_name, installation_id
	`

	var linked app.Repo
	var githubRepoID *int64
	var githubFullName *string
	var installationID *int64
	if repo.GitHubRepoID != 0 {
		githubRepoID = &repo.GitHubRepoID
	}
	if repo.GitHubFullName != "" {
		githubFullName = &repo.GitHubFullName
	}
	if repo.InstallationID != 0 {
		installationID = &repo.InstallationID
	}

	err := s.pool.QueryRow(ctx, q,
		appID, repo.URL, repo.Provider, repo.Branch,
		githubRepoID, githubFullName, installationID,
	).Scan(
		&linked.URL, &linked.Provider, &linked.Branch,
		&githubRepoID, &githubFullName, &installationID,
	)
	if err != nil {
		return app.Repo{}, fmt.Errorf("link app repo: %w", err)
	}
	if githubRepoID != nil {
		linked.GitHubRepoID = *githubRepoID
	}
	if githubFullName != nil {
		linked.GitHubFullName = *githubFullName
	}
	if installationID != nil {
		linked.InstallationID = *installationID
	}
	return linked, nil
}

func (s *Store) GetRepo(ctx context.Context, appID string) (app.Repo, error) {
	const q = `
		SELECT url, provider, branch, github_repo_id, github_full_name, installation_id
		FROM app_repos
		WHERE app_id = $1
	`

	var repo app.Repo
	var githubRepoID *int64
	var githubFullName *string
	var installationID *int64
	err := s.pool.QueryRow(ctx, q, appID).Scan(
		&repo.URL, &repo.Provider, &repo.Branch,
		&githubRepoID, &githubFullName, &installationID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Repo{}, ErrRepoNotFound
	}
	if err != nil {
		return app.Repo{}, fmt.Errorf("get app repo: %w", err)
	}
	if githubRepoID != nil {
		repo.GitHubRepoID = *githubRepoID
	}
	if githubFullName != nil {
		repo.GitHubFullName = *githubFullName
	}
	if installationID != nil {
		repo.InstallationID = *installationID
	}
	return repo, nil
}

func (s *Store) FindAppByRepo(ctx context.Context, url, branch string) (string, error) {
	const q = `
		SELECT app_id::text
		FROM app_repos
		WHERE url = $1 AND branch = $2
	`

	var appID string
	err := s.pool.QueryRow(ctx, q, url, branch).Scan(&appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrRepoNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find app by repo: %w", err)
	}
	return appID, nil
}

func (s *Store) FindAppByGitHubRepoID(ctx context.Context, repoID int64, branch string) (string, error) {
	const q = `
		SELECT app_id::text
		FROM app_repos
		WHERE github_repo_id = $1 AND branch = $2
	`

	var appID string
	err := s.pool.QueryRow(ctx, q, repoID, branch).Scan(&appID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrRepoNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find app by github repo id: %w", err)
	}
	return appID, nil
}

func (s *Store) UnlinkRepo(ctx context.Context, appID string) error {
	const q = `DELETE FROM app_repos WHERE app_id = $1`

	tag, err := s.pool.Exec(ctx, q, appID)
	if err != nil {
		return fmt.Errorf("unlink app repo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRepoNotFound
	}
	return nil
}
