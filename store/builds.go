package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pixll/atlas/build"
)

var ErrBuildNotFound = errors.New("build not found")

func (s *Store) CreateBuild(ctx context.Context, appID string) (build.Build, error) {
	const q = `
		INSERT INTO builds (app_id, status)
		VALUES ($1, $2)
		RETURNING id, app_id, status, created_at, updated_at
	`

	var b build.Build
	err := s.pool.QueryRow(ctx, q, appID, build.StatusPending).
		Scan(&b.ID, &b.AppID, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return build.Build{}, fmt.Errorf("insert build: %w", err)
	}
	return b, nil
}

func (s *Store) GetBuild(ctx context.Context, id string) (build.Build, error) {
	const q = `
		SELECT id, app_id, status, created_at, updated_at
		FROM builds
		WHERE id = $1
	`

	var b build.Build
	err := s.pool.QueryRow(ctx, q, id).Scan(&b.ID, &b.AppID, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return build.Build{}, ErrBuildNotFound
	}
	if err != nil {
		return build.Build{}, fmt.Errorf("get build: %w", err)
	}
	return b, nil
}

func (s *Store) ListBuildsByApp(ctx context.Context, appID string) ([]build.Build, error) {
	const q = `
		SELECT id, app_id, status, created_at, updated_at
		FROM builds
		WHERE app_id = $1
		ORDER BY created_at DESC
	`

	rows, err := s.pool.Query(ctx, q, appID)
	if err != nil {
		return nil, fmt.Errorf("list builds: %w", err)
	}
	defer rows.Close()

	builds := make([]build.Build, 0)
	for rows.Next() {
		var b build.Build
		if err := rows.Scan(&b.ID, &b.AppID, &b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan build: %w", err)
		}
		builds = append(builds, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list builds: %w", err)
	}
	return builds, nil
}

func (s *Store) UpdateBuildStatus(ctx context.Context, id string, status build.Status) (build.Build, error) {
	const q = `
		UPDATE builds
		SET status = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, app_id, status, created_at, updated_at
	`

	var b build.Build
	err := s.pool.QueryRow(ctx, q, id, status).
		Scan(&b.ID, &b.AppID, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return build.Build{}, ErrBuildNotFound
	}
	if err != nil {
		return build.Build{}, fmt.Errorf("update build status: %w", err)
	}
	return b, nil
}
