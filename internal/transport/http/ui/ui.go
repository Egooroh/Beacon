// Package ui embeds the static web dashboard files.
package ui

import "embed"

//go:embed index.html
var Files embed.FS
