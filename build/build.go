package build

import "time"

// Status is the lifecycle state of a build.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

// Phase is a finer-grained step while a build is running.
type Phase string

const (
	PhaseQueued    Phase = "queued"
	PhaseCloning   Phase = "cloning"
	PhaseBuilding  Phase = "building"
	PhasePushing   Phase = "pushing"
	PhaseDeploying Phase = "deploying"
)

// Build is a single attempt to build and package an App from source.
type Build struct {
	ID        string
	AppID     string
	Status    Status
	Phase     Phase
	Image     string
	Log       string
	CreatedAt time.Time
	UpdatedAt time.Time
}
