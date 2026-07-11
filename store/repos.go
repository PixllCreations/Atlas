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
		INSERT INTO app_repos (app_id, url, provider, branch)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (app_id) DO UPDATE
		SET url = EXCLUDED.url,
			provider = EXCLUDED.provider,
			branch = EXCLUDED.branch,
			updated_at = now()
		RETURNING url, provider, branch
	`

	var linked app.Repo
	err := s.pool.QueryRow(ctx, q, appID, repo.URL, repo.Provider, repo.Branch).
		Scan(&linked.URL, &linked.Provider, &linked.Branch)
	if err != nil {
		return app.Repo{}, fmt.Errorf("link app repo: %w", err)
	}
	return linked, nil
}

func (s *Store) GetRepo(ctx context.Context, appID string) (app.Repo, error) {
	const q = `
		SELECT url, provider, branch
		FROM app_repos
		WHERE app_id = $1
	`

	var repo app.Repo
	err := s.pool.QueryRow(ctx, q, appID).Scan(&repo.URL, &repo.Provider, &repo.Branch)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Repo{}, ErrRepoNotFound
	}
	if err != nil {
		return app.Repo{}, fmt.Errorf("get app repo: %w", err)
	}
	return repo, nil
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
