package web

import "embed"

// Files contains all runtime templates, content, and static assets.
//
//go:embed templates content static
var Files embed.FS
