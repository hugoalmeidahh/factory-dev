//go:build release

package web

import "embed"

//go:embed templates/* templates/partials/* templates/ssh/* templates/repos/* static/* static/css/* static/js/*
var FS embed.FS
