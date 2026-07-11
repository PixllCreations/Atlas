package build

import (
	"context"
	"fmt"
)

type Store interface {
	GetBuild(ctx context.Context, id string) (Build, error)
	UpdateBuildStatus(ctx context.Context, id string, status Status) (Build, error)
}

// Worker executes builds and updates their lifecycle status.
type Worker struct {
	store Store
}

func NewWorker(store Store) *Worker {
	return &Worker{store: store}
}

// Process runs a single build by ID.
func (w *Worker) Process(ctx context.Context, buildID string) error {
	b, err := w.store.GetBuild(ctx, buildID)
	if err != nil {
		return fmt.Errorf("get build: %w", err)
	}
	if b.Status != StatusPending {
		return nil
	}

	if _, err := w.store.UpdateBuildStatus(ctx, buildID, StatusRunning); err != nil {
		return fmt.Errorf("mark running: %w", err)
	}

	if err := w.execute(ctx, b); err != nil {
		if _, updateErr := w.store.UpdateBuildStatus(ctx, buildID, StatusFailed); updateErr != nil {
			return fmt.Errorf("run build: %w (mark failed: %v)", err, updateErr)
		}
		return fmt.Errorf("run build: %w", err)
	}

	if _, err := w.store.UpdateBuildStatus(ctx, buildID, StatusSucceeded); err != nil {
		return fmt.Errorf("mark succeeded: %w", err)
	}
	return nil
}

func (w *Worker) execute(ctx context.Context, b Build) error {
	_ = ctx
	_ = b
	// Clone, docker build, and registry push come next.
	return nil
}
