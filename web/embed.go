package web

import "embed"

// Dist holds the production frontend build output.
//
//go:embed all:dist
var Dist embed.FS
