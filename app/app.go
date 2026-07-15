package app

import "time"

// DefaultPort is the container port Atlas assumes when none is set.
const DefaultPort = 80

// App is a deployable application registered with Atlas.
type App struct {
	ID                 string
	Name               string
	Port               int    // container listen port (settings fallback; deploy uses atlas.yaml)
	DeploymentSnapshot []byte // last successful deploy infrastructure JSON; nil if never deployed
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
