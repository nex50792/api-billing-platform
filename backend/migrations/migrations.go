package migrations

import "embed"

//go:embed 000*.sql
var FS embed.FS
