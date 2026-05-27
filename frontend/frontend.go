package frontend

import "embed"

//go:embed all:dist
var DistDir embed.FS
