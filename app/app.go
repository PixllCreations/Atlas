package app

import "time"

// App is a deployable application registered with Atlas.
type App struct {
	ID                 string
	Name               string
	DeploymentSnapshot []byte // last successful deploy infrastructure JSON; nil if never deployed
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
