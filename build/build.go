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

// Build is a single attempt to build and package an App from source.
type Build struct {
	ID        string
	AppID     string
	Status    Status
	Image     string
	CreatedAt time.Time
	UpdatedAt time.Time
}
