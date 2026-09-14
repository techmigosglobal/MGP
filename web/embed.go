package web

import "embed"

// Static contains the compiled browser assets shipped with the server.
//
//go:embed static/*
var Static embed.FS
