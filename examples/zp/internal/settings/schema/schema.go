// Package schema is zp's configuration schema, from interface.md §3.1,
// without the store and mutable keys, which M7.9 adds.
package schema

import (
	"runtime"
	"time"

	"github.com/epistemer/cfgx/dsl"

	"zp/internal/validate"
	"zp/models"
)

// Tags: cross-cutting sets of keys that become interfaces.
var (
	Mailer = dsl.Tag("Mailer", "Everything the mailer needs")
	Tokens = dsl.Tag("Tokens", "Signing keys and lifetimes")
)

var Log = dsl.Group("log", "Logging",
	dsl.Enum("level", "Minimum log level", "debug", "info", "warn", "error").Default("info"),
	dsl.Enum("format", "Output format", "text", "json").Default("text"),
	dsl.Bool("color", "Force colored output even without a TTY"),
)

var DB = dsl.Group("database", "Database connection",
	dsl.String("host", "Database server host").Default("localhost"),
	dsl.Int("port", "Database server port").Default(5432).Range(1, 65535),
	dsl.String("name", "Database name").Required(),
	dsl.String("user", "Database user").Default("postgres"),
	dsl.Secret("password", "Database password").Required(),
	dsl.Enum("sslmode", "SSL mode", "disable", "require", "verify-full").
		Default("disable").Alias("database.ssl_mode"),
).GoName("DB").Computed("DSN")

var SMTP = dsl.Group("smtp", "Outgoing mail",
	dsl.String("host", "SMTP server host").Required(),
	dsl.Int("port", "SMTP server port").Default(587),
	dsl.String("user", "SMTP user"),
	dsl.Secret("password", "SMTP password"),
	dsl.Enum("auth", "SMTP auth mechanism", "PLAIN", "LOGIN", "CRAM-MD5").Default("PLAIN"),
	dsl.Bool("tls", "Use implicit TLS"),
).Tags(Mailer)

var (
	apiToken = dsl.Secret("token", "Static API token")
	apiKeys  = dsl.SecretFile("keys", "CSV file of API keys").Complete(dsl.Files("*.csv"))
)

var Auth = dsl.Group("auth", "Authentication",
	// The key stays emailfrom in the file; GoName makes the getter
	// AuthEmailFrom(), as interface.md §5.1 has it.
	dsl.String("emailfrom", "Sender address for auth emails").
		Validate(validate.Email).GoName("EmailFrom").Tags(Mailer),
	dsl.Duration("otp.ttl", "OTP validity").Default(5*time.Minute),
	dsl.Int("otp.length", "OTP length").Default(6).Range(4, 10),
	dsl.Duration("session.ttl", "Session lifetime").Default(24*time.Hour).Tags(Tokens),
	dsl.SecretFile("jwt.private", "JWT private key (PEM)").Default("keys/priv.pem").Tags(Tokens),
	dsl.SecretFile("jwt.public", "JWT public key (PEM)").Default("keys/pub.pem").Tags(Tokens),
	apiToken,
	apiKeys,
	dsl.Bool("mfa", "Require multi-factor authentication"),
).Rules(
	dsl.AtMostOne(apiToken, apiKeys),
)

var (
	tlsEnabled = dsl.Bool("tls.enabled", "Serve HTTPS")
	tlsCert    = dsl.Path("tls.cert", "TLS certificate").Default("ssl/cert.pem").Complete(dsl.Files("*.pem"))
	tlsKey     = dsl.SecretFile("tls.key", "TLS private key").Default("ssl/key.pem")
)

var Server = dsl.Group("server", "HTTP server",
	dsl.String("host", "Listen address").Default("0.0.0.0"),
	dsl.Int("port", "Listen port").Default(7777).Range(1, 65535).Short('p'),
	tlsEnabled, tlsCert, tlsKey,
	dsl.Duration("timeout.read", "Read timeout").Default(20*time.Second),
	dsl.Duration("timeout.write", "Write timeout").Default(60*time.Second),
	dsl.ByteSize("max_header", "Maximum request header size").Default(1<<20),
	dsl.Int("rate.limit", "Requests per second per IP").Default(100),
	dsl.Int("rate.burst", "Burst capacity per IP").Default(20),
).FlagPrefix("").Rules(
	dsl.RelevantIf(tlsCert, tlsEnabled),
	dsl.RelevantIf(tlsKey, tlsEnabled),
)

var Scheduler = dsl.Group("scheduler", "Task scheduler",
	dsl.Location("location", "Scheduler timezone").Default("Africa/Cairo"),
	dsl.Enum("execmode", "Task execution mode",
		models.ZpTaskExecutionModeSequential,
		models.ZpTaskExecutionModeParallel,
	).Default(models.ZpTaskExecutionModeSequential),
	dsl.Int("workers", "Concurrent workers").DefaultFunc(runtime.NumCPU),
	dsl.Group("retry", "Retry policy",
		dsl.Int("max", "Maximum retries").Default(3),
		dsl.Duration("delay", "Delay between retries").Default(time.Minute),
	),
).Computed("RetryPolicy")

var boltFile = dsl.Path("bolt", "Bolt database file; also holds runtime settings").Default("bolt.db")

var Storage = dsl.Group("storage", "Local storage", boltFile)

// The application.
var App = dsl.App("zp", "ZaynPay backend",
	Log, DB, SMTP, Auth, Server, Scheduler, Storage,
).
	EnvPrefix("ZP").
	ConfigFile("config", ".", "$HOME/.zp", "/etc/zp").
	Initialisms("OTP", "JWT", "SMTP", "MFA").
	Global(Log)

// Commands select settings. They are config profiles, not cobra commands.
var (
	Serve = dsl.Command("Serve").Uses(DB, SMTP, Auth, Server, Scheduler)

	Migrate = dsl.Command("Migrate").Uses(DB).Args(
		dsl.Bool("down", "Roll back instead of applying"),
		dsl.Int("steps", "Number of migrations; 0 means all").Min(0),
	)

	TestEmail = dsl.Command("TestEmail").Uses(Mailer).Args(
		dsl.String("to.name", "Recipient name").Default("John Doe"),
		dsl.String("to.address", "Recipient address").Required().Validate(validate.Email),
		dsl.String("template", "Template to render").Default("new-account"),
	)
)
