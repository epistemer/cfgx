package dsl_test

import (
	"time"

	"github.com/epistemer/cfgx/dsl"
)

// A schema declares groups of keys, the app, and the commands that use
// them. It lives in its own package, conventionally
// internal/settings/schema, and the settings package next to it runs
// "go tool cfgx gen ./schema" from a go:generate line. In a schema every
// declaration is a package-level variable; this example declares them in a
// function only so that godoc can show them together.
func Example() {
	var mailer = dsl.Tag("Mailer", "Everything the mailer needs")

	var server = dsl.Group("server", "HTTP server",
		dsl.String("addr", "Address to listen on").Default("localhost:8080"),
		dsl.Duration("timeout", "Request timeout").Default(30*time.Second).Min(time.Second),
		dsl.Int("rate.limit", "Requests per second").Default(100).Min(1).Mutable(),
	)

	var smtp = dsl.Group("smtp", "Outgoing mail",
		dsl.String("host", "SMTP server").Required(),
		dsl.Secret("password", "SMTP password"),
	).Tags(mailer)

	var bolt = dsl.Path("bolt", "Store file for runtime settings").Default("app.db")

	var _ = dsl.App("app", "Example app", server, smtp, dsl.Group("storage", "Storage", bolt)).
		ConfigFile("app", ".", "$HOME/.config/app").
		Store(bolt)

	var _ = dsl.Command("Serve").Uses(server, smtp)
	var _ = dsl.Command("TestEmail").Uses(mailer).Args(
		dsl.String("to", "Recipient").Required(),
	)
}

// Rules relate keys to each other; add them to a group.
func ExampleRelevantIf() {
	tlsOn := dsl.Bool("tls.enabled", "Serve HTTPS")
	cert := dsl.Path("tls.cert", "TLS certificate").Default("cert.pem")
	key := dsl.SecretFile("tls.key", "TLS private key").Default("key.pem")

	var _ = dsl.Group("server", "HTTP server", tlsOn, cert, key).Rules(
		// The certificate and key are read only when TLS is on.
		dsl.RelevantIf(cert, tlsOn),
		dsl.RelevantIf(key, tlsOn),
	)
}

// Enum infers the getter's type from its values. Plain strings give a
// string getter:
func ExampleEnum() {
	var _ = dsl.Enum("level", "Minimum log level", "debug", "info", "warn", "error").Default("info")
}

// Mode stands for a named type from the application, such as an enum
// that sqlc generates. It must be exported from a package the generated
// code can import.
type Mode string

const (
	Sequential Mode = "sequential"
	Parallel   Mode = "parallel"
)

// Typed constants give a getter that returns their type, here Mode.
func ExampleEnum_namedType() {
	var _ = dsl.Enum("mode", "Task execution mode", Sequential, Parallel).Default(Sequential)
}
