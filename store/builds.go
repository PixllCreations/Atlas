package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pixll/atlas/build"
)

var ErrBuildNotFound = errors.New("build not found")

func scanBuild(row pgx.Row) (build.Build, error) {
	var b build.Build
	var phase, image, log string
	err := row.Scan(&b.ID, &b.AppID, &b.Status, &phase, &image, &log, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return build.Build{}, err
	}
	b.Phase = build.Phase(phase)
	b.Image = image
	b.Log = log
	return b, nil
}

const buildCols = `id, app_id, status, phase, image, log, created_at, updated_at`

func (s *Store) CreateBuild(ctx context.Context, appID string) (build.Build, error) {
	const q = `
		INSERT INTO builds (app_id, status, phase)
		VALUES ($1, $2, $3)
		RETURNING ` + buildCols + `
	`

	b, err := scanBuild(s.pool.QueryRow(ctx, q, appID, build.StatusPending, build.PhaseQueued))
	if err != nil {
		return build.Build{}, fmt.Errorf("insert build: %w", err)
	}
	return b, nil
}

func (s *Store) GetBuild(ctx context.Context, id string) (build.Build, error) {
	const q = `SELECT ` + buildCols + ` FROM builds WHERE id = $1`

	b, err := scanBuild(s.pool.QueryRow(ctx, q, id))
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
		SELECT ` + buildCols + `
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
		b, err := scanBuild(rows)
		if err != nil {
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
		RETURNING ` + buildCols + `
	`

	b, err := scanBuild(s.pool.QueryRow(ctx, q, id, status))
	if errors.Is(err, pgx.ErrNoRows) {
		return build.Build{}, ErrBuildNotFound
	}
	if err != nil {
		return build.Build{}, fmt.Errorf("update build status: %w", err)
	}
	return b, nil
}

func (s *Store) UpdateBuildPhase(ctx context.Context, id string, phase build.Phase) (build.Build, error) {
	const q = `
		UPDATE builds
		SET phase = $2, updated_at = now()
		WHERE id = $1
		RETURNING ` + buildCols + `
	`

	b, err := scanBuild(s.pool.QueryRow(ctx, q, id, phase))
	if errors.Is(err, pgx.ErrNoRows) {
		return build.Build{}, ErrBuildNotFound
	}
	if err != nil {
		return build.Build{}, fmt.Errorf("update build phase: %w", err)
	}
	return b, nil
}

func (s *Store) UpdateBuildImage(ctx context.Context, id string, image string) (build.Build, error) {
	const q = `
		UPDATE builds
		SET image = $2, updated_at = now()
		WHERE id = $1
		RETURNING ` + buildCols + `
	`

	b, err := scanBuild(s.pool.QueryRow(ctx, q, id, image))
	if errors.Is(err, pgx.ErrNoRows) {
		return build.Build{}, ErrBuildNotFound
	}
	if err != nil {
		return build.Build{}, fmt.Errorf("update build image: %w", err)
	}
	return b, nil
}

// AppendBuildLog appends chunk to the build log, keeping the newest ~256KiB.
func (s *Store) AppendBuildLog(ctx context.Context, id string, chunk string) error {
	if chunk == "" {
		return nil
	}
	const maxLog = 256 * 1024
	const q = `
		UPDATE builds
		SET log = CASE
			WHEN length(log || $2) <= $3 THEN log || $2
			ELSE right(log || $2, $3)
		END,
		updated_at = now()
		WHERE id = $1
	`
	tag, err := s.pool.Exec(ctx, q, id, chunk, maxLog)
	if err != nil {
		return fmt.Errorf("append build log: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBuildNotFound
	}
	return nil
}
