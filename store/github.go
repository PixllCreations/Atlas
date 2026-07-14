package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GitHubInstallation is a GitHub App installation linked to this Atlas instance.
type GitHubInstallation struct {
	ID           int64
	AccountLogin string
	AccountType  string
}

func (s *Store) UpsertInstallation(ctx context.Context, inst GitHubInstallation) error {
	const q = `
		INSERT INTO github_installations (id, account_login, account_type)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE
		SET account_login = EXCLUDED.account_login,
		    account_type = EXCLUDED.account_type,
		    updated_at = now()
	`
	_, err := s.pool.Exec(ctx, q, inst.ID, inst.AccountLogin, inst.AccountType)
	if err != nil {
		return fmt.Errorf("upsert github installation: %w", err)
	}
	return nil
}

func (s *Store) ListInstallations(ctx context.Context) ([]GitHubInstallation, error) {
	const q = `
		SELECT id, account_login, account_type
		FROM github_installations
		ORDER BY account_login
	`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list github installations: %w", err)
	}
	defer rows.Close()

	var out []GitHubInstallation
	for rows.Next() {
		var inst GitHubInstallation
		if err := rows.Scan(&inst.ID, &inst.AccountLogin, &inst.AccountType); err != nil {
			return nil, fmt.Errorf("scan github installation: %w", err)
		}
		out = append(out, inst)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list github installations: %w", err)
	}
	return out, nil
}

func (s *Store) GetInstallation(ctx context.Context, id int64) (GitHubInstallation, error) {
	const q = `
		SELECT id, account_login, account_type
		FROM github_installations
		WHERE id = $1
	`
	var inst GitHubInstallation
	err := s.pool.QueryRow(ctx, q, id).Scan(&inst.ID, &inst.AccountLogin, &inst.AccountType)
	if errors.Is(err, pgx.ErrNoRows) {
		return GitHubInstallation{}, ErrNotFound
	}
	if err != nil {
		return GitHubInstallation{}, fmt.Errorf("get github installation: %w", err)
	}
	return inst, nil
}

func (s *Store) DeleteInstallation(ctx context.Context, id int64) error {
	const q = `DELETE FROM github_installations WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete github installation: %w", err)
	}
	return nil
}

func (s *Store) UnlinkReposByGitHubIDs(ctx context.Context, repoIDs []int64) error {
	if len(repoIDs) == 0 {
		return nil
	}
	const q = `DELETE FROM app_repos WHERE github_repo_id = ANY($1)`
	_, err := s.pool.Exec(ctx, q, repoIDs)
	if err != nil {
		return fmt.Errorf("unlink repos by github id: %w", err)
	}
	return nil
}
