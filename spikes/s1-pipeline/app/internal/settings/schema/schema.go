// Package schema is a miniature cfgx schema. It imports an internal package of
// the app, as real schemas will.
package schema

import (
	"example.com/app/internal/fakedsl"
	"example.com/app/internal/models"
)

// DB registers itself when the package initializes.
var DB = fakedsl.Group("database",
	fakedsl.String("host"),
	fakedsl.Enum("mode", models.Sequential, models.Parallel),
)
