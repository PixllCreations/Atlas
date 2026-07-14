package app

import "time"

// DefaultPort is the container port Atlas assumes when none is set.
const DefaultPort = 80

// App is a deployable application registered with Atlas.
type App struct {
	ID        string
	Name      string
	Port      int // container listen port
	CreatedAt time.Time
	UpdatedAt time.Time
}
