// Package ui embeds the static web dashboard files.
package ui

import "embed"

//go:embed index.html
// Files holds the embedded static web dashboard assets.
var Files embed.FS
