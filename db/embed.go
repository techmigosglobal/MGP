package db

import "embed"

// Migrations contains the versioned PostgreSQL schema shipped with the server.
//
//go:embed migrations/*.sql
var Migrations embed.FS
