package schemas

import "embed"

// FS embeds the public OpenExit JSON Schemas so release binaries can validate
// manifests without needing a checked-out schemas/ directory.
//
//go:embed *.schema.json
var FS embed.FS
