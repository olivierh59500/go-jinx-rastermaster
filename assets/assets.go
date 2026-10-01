// Package assets embeds native presentation data and YM soundtracks.
package assets

import "embed"

//go:embed original/*
var Files embed.FS
