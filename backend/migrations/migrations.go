// Package migrations embeds all numbered SQL migration files for use by
// the sequential migration runner.
package migrations

import "embed"

// FS contains every *.sql file in this directory, sorted by name when iterated.
//
//go:embed *.sql
var FS embed.FS
