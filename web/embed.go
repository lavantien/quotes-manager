// Package web embeds the HTML templates and static assets so the server ships
// as a single binary.
package web

import "embed"

// Templates holds the HTML templates under web/templates.
//
//go:embed templates/*.html
var Templates embed.FS

// Static holds the static assets under web/static.
//
//go:embed static
var Static embed.FS
