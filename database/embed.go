// Package database embeds the generated seed.sql so the server can load the
// canonical quotes from a single source of truth without a runtime file path.
package database

import _ "embed"

// SeedSQL is the committed, generated database/seed.sql (schema + inserts).
//
//go:embed seed.sql
var SeedSQL string
