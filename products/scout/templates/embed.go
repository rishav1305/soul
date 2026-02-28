// Package templates embeds the HTML template files for resume and cover
// letter generation. Other packages import this to access the embedded FS.
package templates

import "embed"

//go:embed *.html
var FS embed.FS
