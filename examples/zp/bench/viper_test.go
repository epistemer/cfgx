package bench

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// The viper side of each benchmark does what the cfgx side does, written
// the way a viper app usually is: defaults, env bindings and flag bindings
// for every key, the config file, then every value read into a struct and
// checked by hand. It is the code cfgx generates for you.

// vkey is a key with its flag name ("" for none) and default (nil for none).
type vkey struct {
	key, flag string
	def       any
}

// zpKeys are the 39 keys zp's Serve command loads, as viper would see them.
var zpKeys = []vkey{
	{"log.level", "log-level", "info"},
	{"log.format", "log-format", "text"},
	{"log.color", "log-color", false},
	{"database.host", "database-host", "localhost"},
	{"database.port", "database-port", 5432},
	{"database.name", "database-name", ""},
	{"database.user", "database-user", "postgres"},
	{"database.password", "", nil},
	{"database.sslmode", "database-sslmode", "disable"},
	{"smtp.host", "smtp-host", ""},
	{"smtp.port", "smtp-port", 587},
	{"smtp.user", "smtp-user", ""},
	{"smtp.password", "", nil},
	{"smtp.auth", "smtp-auth", "PLAIN"},
	{"smtp.tls", "smtp-tls", false},
	{"auth.emailfrom", "auth-emailfrom", ""},
	{"auth.otp.ttl", "auth-otp-ttl", 5 * time.Minute},
	{"auth.otp.length", "auth-otp-length", 6},
	{"auth.session.ttl", "auth-session-ttl", 24 * time.Hour},
	{"auth.jwt.private", "auth-jwt-private", "keys/priv.pem"},
	{"auth.jwt.public", "auth-jwt-public", "keys/pub.pem"},
	{"auth.token", "", nil},
	{"auth.keys", "auth-keys", ""},
	{"auth.mfa", "auth-mfa", false},
	{"server.host", "host", "0.0.0.0"},
	{"server.port", "port", 7777},
	{"server.tls.enabled", "tls-enabled", false},
	{"server.tls.cert", "tls-cert", "ssl/cert.pem"},
	{"server.tls.key", "tls-key", "ssl/key.pem"},
	{"server.timeout.read", "timeout-read", 20 * time.Second},
	{"server.timeout.write", "timeout-write", time.Minute},
	{"server.max_header", "max-header", "1mb"},
	{"server.rate.limit", "rate-limit", 100},
	{"server.rate.burst", "rate-burst", 20},
	{"scheduler.location", "scheduler-location", "Africa/Cairo"},
	{"scheduler.execmode", "scheduler-execmode", "sequential"},
	{"scheduler.workers", "scheduler-workers", runtime.NumCPU()},
	{"scheduler.retry.max", "scheduler-retry-max", 3},
	{"scheduler.retry.delay", "scheduler-retry-delay", time.Minute},
}

// newViper sets up aliases, defaults, env bindings and flags for keys,
// parses args and reads the config file.
func newViper(prefix string, keys []vkey, aliases map[string]string, args []string, file string) (*viper.Viper, error) {
	v := viper.New()
	v.SetEnvPrefix(prefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	for alias, key := range aliases {
		v.RegisterAlias(alias, key)
	}
	fs := pflag.NewFlagSet("bench", pflag.ContinueOnError)
	for _, k := range keys {
		if k.def != nil {
			v.SetDefault(k.key, k.def)
		}
		if err := v.BindEnv(k.key); err != nil {
			return nil, err
		}
		if k.flag == "" {
			continue
		}
		switch d := k.def.(type) {
		case bool:
			fs.Bool(k.flag, d, k.key)
		case int:
			fs.Int(k.flag, d, k.key)
		case time.Duration:
			fs.Duration(k.flag, d, k.key)
		case string:
			fs.String(k.flag, d, k.key)
		}
		if err := v.BindPFlag(k.key, fs.Lookup(k.flag)); err != nil {
			return nil, err
		}
	}
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	v.SetConfigFile(file)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	return v, nil
}

type zpServe struct {
	LogLevel, LogFormat                           string
	LogColor                                      bool
	DBHost, DBName, DBUser, DBPassword, DBSSLMode string
	DBPort                                        int
	SMTPHost, SMTPUser, SMTPPassword, SMTPAuth    string
	SMTPPort                                      int
	SMTPTLS                                       bool
	AuthEmailFrom, AuthToken                      string
	AuthOTPTTL, AuthSessionTTL                    time.Duration
	AuthOTPLength                                 int
	AuthJWTPrivate, AuthJWTPublic, AuthKeys       []byte
	AuthMFA                                       bool
	ServerHost, ServerTLSCert                     string
	ServerPort, ServerRateLimit, ServerRateBurst  int
	ServerTLSEnabled                              bool
	ServerTLSKey                                  []byte
	ServerReadTimeout, ServerWriteTimeout         time.Duration
	ServerMaxHeader                               uint
	SchedulerLocation                             *time.Location
	SchedulerExecMode                             string
	SchedulerWorkers, SchedulerRetryMax           int
	SchedulerRetryDelay                           time.Duration
}

func oneOf(key, s string, allowed ...string) error {
	if !slices.Contains(allowed, s) {
		return fmt.Errorf("%s: %q isn't one of %s", key, s, strings.Join(allowed, ", "))
	}
	return nil
}

func between(key string, n, lo, hi int) error {
	if n < lo || n > hi {
		return fmt.Errorf("%s: must be between %d and %d", key, lo, hi)
	}
	return nil
}

func required(key, s string) error {
	if s == "" {
		return fmt.Errorf("%s: is required", key)
	}
	return nil
}

// readSecretFile reads a secret file key's file, if it names one.
func readSecretFile(v *viper.Viper, key string) ([]byte, error) {
	p := v.GetString(key)
	if p == "" {
		return nil, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return b, nil
}

// viperServe loads zp's Serve configuration with viper.
func viperServe(args []string, file string) (*zpServe, error) {
	v, err := newViper("ZP", zpKeys, map[string]string{"database.ssl_mode": "database.sslmode"}, args, file)
	if err != nil {
		return nil, err
	}
	c := &zpServe{
		LogLevel: v.GetString("log.level"), LogFormat: v.GetString("log.format"), LogColor: v.GetBool("log.color"),
		DBHost: v.GetString("database.host"), DBPort: v.GetInt("database.port"), DBName: v.GetString("database.name"),
		DBUser: v.GetString("database.user"), DBPassword: v.GetString("database.password"), DBSSLMode: v.GetString("database.sslmode"),
		SMTPHost: v.GetString("smtp.host"), SMTPPort: v.GetInt("smtp.port"), SMTPUser: v.GetString("smtp.user"),
		SMTPPassword: v.GetString("smtp.password"), SMTPAuth: v.GetString("smtp.auth"), SMTPTLS: v.GetBool("smtp.tls"),
		AuthEmailFrom: v.GetString("auth.emailfrom"), AuthOTPTTL: v.GetDuration("auth.otp.ttl"),
		AuthOTPLength: v.GetInt("auth.otp.length"), AuthSessionTTL: v.GetDuration("auth.session.ttl"),
		AuthToken: v.GetString("auth.token"), AuthMFA: v.GetBool("auth.mfa"),
		ServerHost: v.GetString("server.host"), ServerPort: v.GetInt("server.port"),
		ServerTLSEnabled: v.GetBool("server.tls.enabled"), ServerTLSCert: v.GetString("server.tls.cert"),
		ServerReadTimeout: v.GetDuration("server.timeout.read"), ServerWriteTimeout: v.GetDuration("server.timeout.write"),
		ServerMaxHeader: v.GetSizeInBytes("server.max_header"),
		ServerRateLimit: v.GetInt("server.rate.limit"), ServerRateBurst: v.GetInt("server.rate.burst"),
		SchedulerExecMode: v.GetString("scheduler.execmode"), SchedulerWorkers: v.GetInt("scheduler.workers"),
		SchedulerRetryMax: v.GetInt("scheduler.retry.max"), SchedulerRetryDelay: v.GetDuration("scheduler.retry.delay"),
	}
	errs := []error{
		oneOf("log.level", c.LogLevel, "debug", "info", "warn", "error"),
		oneOf("log.format", c.LogFormat, "text", "json"),
		between("database.port", c.DBPort, 1, 65535),
		required("database.name", c.DBName),
		required("database.password", c.DBPassword),
		oneOf("database.sslmode", c.DBSSLMode, "disable", "require", "verify-full"),
		required("smtp.host", c.SMTPHost),
		oneOf("smtp.auth", c.SMTPAuth, "PLAIN", "LOGIN", "CRAM-MD5"),
		between("auth.otp.length", c.AuthOTPLength, 4, 10),
		between("server.port", c.ServerPort, 1, 65535),
		oneOf("scheduler.execmode", c.SchedulerExecMode, "sequential", "parallel"),
	}
	if c.AuthEmailFrom != "" {
		if _, err := mail.ParseAddress(c.AuthEmailFrom); err != nil {
			errs = append(errs, fmt.Errorf("auth.emailfrom: %q isn't an email address", c.AuthEmailFrom))
		}
	}
	if c.AuthToken != "" && v.GetString("auth.keys") != "" {
		errs = append(errs, errors.New("auth.token and auth.keys: set at most one"))
	}
	if c.SchedulerLocation, err = time.LoadLocation(v.GetString("scheduler.location")); err != nil {
		errs = append(errs, fmt.Errorf("scheduler.location: %w", err))
	}
	for _, f := range []struct {
		key string
		dst *[]byte
		on  bool
	}{
		{"auth.jwt.private", &c.AuthJWTPrivate, true},
		{"auth.jwt.public", &c.AuthJWTPublic, true},
		{"auth.keys", &c.AuthKeys, true},
		{"server.tls.key", &c.ServerTLSKey, c.ServerTLSEnabled},
	} {
		if f.on {
			var err error
			*f.dst, err = readSecretFile(v, f.key)
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return c, nil
}

// keys100 are the 100 keys of the keys100 schema.
var keys100 = func() []vkey {
	var ks []vkey
	for g := range 10 {
		for k := range 10 {
			key, flag := fmt.Sprintf("g%d.k%d", g, k), fmt.Sprintf("g%d-k%d", g, k)
			var def any
			switch k % 4 {
			case 0:
				def = k
			case 1:
				def = "v"
			case 2:
				def = time.Duration(k) * time.Second
			case 3:
				def = false
			}
			ks = append(ks, vkey{key, flag, def})
		}
	}
	return ks
}()

type values100 struct {
	Ints      [10][3]int           // k0, k4, k8
	Strings   [10][3]string        // k1, k5, k9
	Durations [10][2]time.Duration // k2, k6
	Bools     [10][2]bool          // k3, k7
}

// viper100 loads the 100 keys with viper.
func viper100(args []string, file string) (*values100, error) {
	v, err := newViper("BENCH", keys100, nil, args, file)
	if err != nil {
		return nil, err
	}
	var c values100
	var errs []error
	for g := range 10 {
		var i, s, d, b int
		for k := range 10 {
			key := keys100[g*10+k].key
			switch k % 4 {
			case 0:
				n := v.GetInt(key)
				if n < 0 {
					errs = append(errs, fmt.Errorf("%s: must be at least 0", key))
				}
				c.Ints[g][i], i = n, i+1
			case 1:
				c.Strings[g][s], s = v.GetString(key), s+1
			case 2:
				c.Durations[g][d], d = v.GetDuration(key), d+1
			case 3:
				c.Bools[g][b], b = v.GetBool(key), b+1
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return &c, nil
}
