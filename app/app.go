package app

import "time"

// App is a deployable application registered with Atlas.
type App struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
