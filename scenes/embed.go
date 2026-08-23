package scenes

import "embed"

// Files contains the scene definitions shipped with CLI Quest.
//
//go:embed linux/*.json git/*.json process/*.json http/*.json
var Files embed.FS
