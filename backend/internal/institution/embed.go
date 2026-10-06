package institution

import "embed"

//go:embed migrations/*.sql
var MigrationsFS embed.FS

// MigrationsDir is the directory inside MigrationsFS containing the SQL files.
const MigrationsDir = "migrations"
