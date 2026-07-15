package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pixll/atlas/app"
)

var ErrNotFound = errors.New("app not found")

func (s *Store) CreateApp(ctx context.Context, name string) (app.App, error) {
	const q = `
		INSERT INTO apps (name)
		VALUES ($1)
		RETURNING id, name, deployment_snapshot, created_at, updated_at
	`

	var a app.App
	err := s.pool.QueryRow(ctx, q, name).Scan(&a.ID, &a.Name, &a.DeploymentSnapshot, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return app.App{}, fmt.Errorf("insert app: %w", err)
	}
	return a, nil
}

func (s *Store) GetApp(ctx context.Context, id string) (app.App, error) {
	const q = `
		SELECT id, name, deployment_snapshot, created_at, updated_at
		FROM apps
		WHERE id = $1
	`

	var a app.App
	err := s.pool.QueryRow(ctx, q, id).Scan(&a.ID, &a.Name, &a.DeploymentSnapshot, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.App{}, ErrNotFound
	}
	if err != nil {
		return app.App{}, fmt.Errorf("get app: %w", err)
	}
	return a, nil
}

func (s *Store) ListApps(ctx context.Context) ([]app.App, error) {
	const q = `
		SELECT id, name, deployment_snapshot, created_at, updated_at
		FROM apps
		ORDER BY created_at
	`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list apps: %w", err)
	}
	defer rows.Close()

	apps := make([]app.App, 0)
	for rows.Next() {
		var a app.App
		if err := rows.Scan(&a.ID, &a.Name, &a.DeploymentSnapshot, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan app: %w", err)
		}
		apps = append(apps, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list apps: %w", err)
	}
	return apps, nil
}

// UpdateAppDeploymentSnapshot stores the last successful deploy infrastructure snapshot.
func (s *Store) UpdateAppDeploymentSnapshot(ctx context.Context, id string, snapshot []byte) error {
	const q = `
		UPDATE apps
		SET deployment_snapshot = $2, updated_at = now()
		WHERE id = $1
	`
	tag, err := s.pool.Exec(ctx, q, id, snapshot)
	if err != nil {
		return fmt.Errorf("update deployment snapshot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteApp(ctx context.Context, id string) error {
	const q = `DELETE FROM apps WHERE id = $1`

	tag, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete app: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
