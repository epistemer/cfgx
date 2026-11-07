// Package schema declares the configuration schema for the mini example.
package schema

import "github.com/epistemer/cfgx/dsl"

// Server settings configure the HTTP listener.
var Server = dsl.Group("server", "HTTP server",
	dsl.String("host", "Listen host").Default("localhost"),
	dsl.Int("port", "Listen port").Default(8080),
	dsl.Bool("tls", "Enable TLS"),
)

// Database settings configure the database connection.
var Database = dsl.Group("database", "Database connection",
	dsl.String("url", "Database connection URL"),
	dsl.Int("timeout", "Query timeout in seconds"),
	dsl.Bool("debug", "Log SQL queries").Default(false),
)

// App is the root schema declaration for mini.
var App = dsl.App("mini", "Minimal cfgx example",
	Server,
	Database,
).EnvPrefix("MINI")

// Commands declare which groups each CLI command uses.
var (
	// Serve uses both Server and Database settings.
	Serve = dsl.Command("Serve").Uses(Server, Database)

	// Migrate uses only Database settings.
	Migrate = dsl.Command("Migrate").Uses(Database)
)
