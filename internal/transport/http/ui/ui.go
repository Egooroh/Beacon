// Package ui embeds the static web dashboard files.
package ui

import "embed"

// Files holds the embedded static web dashboard assets.
//
//go:embed index.html
var Files embed.FS
