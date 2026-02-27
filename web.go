package soul

import "embed"

// WebDist holds the built React SPA files from web/dist/.
// This is populated at build time by go:embed.
// When web/dist/ doesn't exist (dev mode or before first build),
// the embed will be empty and the server falls back to the placeholder.
//
//go:embed all:web/dist
var WebDist embed.FS
