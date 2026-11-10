// Package bench compares cfgx with viper (M9.1). It is its own module so
// that viper never becomes a dependency of cfgx or of the zp example; the
// results are in README.md.
package bench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/epistemer/cfgx"

	settings100 "zp/bench/keys100"
	"zp/internal/settings"
)

// zpFile is the config file both zp loads read; env and flags set a few
// more keys, and the rest keep their defaults.
const zpFile = `database:
  name: zp
  host: db.internal
  sslmode: require
smtp:
  host: smtp.internal
  auth: LOGIN
auth:
  emailfrom: auth@example.com
  otp:
    length: 8
server:
  rate:
    limit: 200
scheduler:
  location: Europe/Berlin
  retry:
    delay: 30s
`

var zpArgs = []string{"--port", "8080", "--log-level", "debug"}

// zpDir makes a directory with the config file and the JWT keys, sets
// the env vars, and moves into it.
func zpDir(b *testing.B) string {
	dir := b.TempDir()
	for name, body := range map[string]string{"config.yaml": zpFile, "keys/priv.pem": "private", "keys/pub.pem": "public"} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	b.Setenv("ZP_DATABASE_PASSWORD", "pw")
	b.Setenv("ZP_SMTP_USER", "mailer")
	b.Chdir(dir)
	return filepath.Join(dir, "config.yaml")
}

// BenchmarkZpServe loads zp's Serve configuration, 39 keys, from a config
// file, env vars, flags and defaults, validated, with secret files read.
func BenchmarkZpServe(b *testing.B) {
	b.Run("cfgx", func(b *testing.B) {
		file := zpDir(b)
		ctx := context.Background()
		b.ReportAllocs()
		for b.Loop() {
			s, err := settings.LoadServe(ctx, cfgx.WithArgs(zpArgs...), cfgx.WithFile(file))
			if err != nil {
				b.Fatal(err)
			}
			if s.ServerPort() != 8080 || s.DBHost() != "db.internal" || s.LogLevel() != "debug" || s.AuthOTPLength() != 8 ||
				s.SMTPUser() != "mailer" || s.DBPassword().Reveal() != "pw" || string(s.AuthJWTPrivate().Reveal()) != "private" {
				b.Fatalf("port %d, host %s, level %s, otp %d, smtp user %s", s.ServerPort(), s.DBHost(), s.LogLevel(), s.AuthOTPLength(), s.SMTPUser())
			}
		}
	})
	b.Run("viper", func(b *testing.B) {
		file := zpDir(b)
		b.ReportAllocs()
		for b.Loop() {
			c, err := viperServe(zpArgs, file)
			if err != nil {
				b.Fatal(err)
			}
			if c.ServerPort != 8080 || c.DBHost != "db.internal" || c.LogLevel != "debug" || c.AuthOTPLength != 8 ||
				c.SMTPUser != "mailer" || c.DBPassword != "pw" || string(c.AuthJWTPrivate) != "private" {
				b.Fatalf("port %d, host %s, level %s, otp %d, smtp user %s", c.ServerPort, c.DBHost, c.LogLevel, c.AuthOTPLength, c.SMTPUser)
			}
		}
	})
}

// keys100Dir writes a config file that sets 30 of the 100 keys and sets
// ten env vars; the arguments set five more.
func keys100Dir(b *testing.B) string {
	var f []byte
	for g := range 10 {
		f = append(f, "g"+string(rune('0'+g))+":\n  k0: 7\n  k1: file\n  k4: 9\n"...)
	}
	file := filepath.Join(b.TempDir(), "bench.yaml")
	if err := os.WriteFile(file, f, 0o600); err != nil {
		b.Fatal(err)
	}
	for g := range 10 {
		b.Setenv("BENCH_G"+string(rune('0'+g))+"_K2", "90s")
	}
	return file
}

var args100 = []string{"--g0-k3", "--g1-k3", "--g2-k3", "--g3-k3", "--g4-k3"}

// BenchmarkKeys100 loads 100 keys: 30 from a config file, 10 from env
// vars, 5 from flags, the rest defaults.
func BenchmarkKeys100(b *testing.B) {
	b.Run("cfgx", func(b *testing.B) {
		file := keys100Dir(b)
		ctx := context.Background()
		b.ReportAllocs()
		for b.Loop() {
			r, err := settings100.LoadRun(ctx, cfgx.WithArgs(args100...), cfgx.WithFile(file))
			if err != nil {
				b.Fatal(err)
			}
			if r.G9K0() != 7 || r.G9K2().Seconds() != 90 || !r.G4K3() || r.G5K3() || r.G9K9() != "v" {
				b.Fatal("wrong values")
			}
		}
	})
	b.Run("viper", func(b *testing.B) {
		file := keys100Dir(b)
		b.ReportAllocs()
		for b.Loop() {
			c, err := viper100(args100, file)
			if err != nil {
				b.Fatal(err)
			}
			if c.Ints[9][0] != 7 || c.Durations[9][0].Seconds() != 90 || !c.Bools[4][0] || c.Bools[5][0] || c.Strings[9][2] != "v" {
				b.Fatal("wrong values")
			}
		}
	})
}

// BenchmarkGenerate times cfgx gen -check on zp's schema: what go generate
// costs on every run. Warm is with the go build cache filled; cold starts
// each run with an empty build cache, as on a fresh machine or CI runner
// (the module cache stays, so nothing is downloaded). Cold runs take
// seconds: use -benchtime=2x. -short skips both.
func BenchmarkGenerate(b *testing.B) {
	if testing.Short() {
		b.Skip("runs the go command")
	}
	bin := filepath.Join(b.TempDir(), "cfgx")
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/epistemer/cfgx/cmd/cfgx").CombinedOutput(); err != nil {
		b.Fatalf("%v\n%s", err, out)
	}
	gen := func(b *testing.B, env ...string) {
		cmd := exec.Command(bin, "gen", "-check", "./schema")
		cmd.Dir = "../internal/settings" // where go generate runs it
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("%v\n%s", err, out)
		}
	}
	b.Run("warm", func(b *testing.B) {
		gen(b) // fills the cache
		for b.Loop() {
			gen(b)
		}
	})
	b.Run("cold", func(b *testing.B) {
		for b.Loop() {
			b.StopTimer()
			cache, err := os.MkdirTemp("", "cfgx-bench-cache-")
			if err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			gen(b, "GOCACHE="+cache)
			b.StopTimer()
			os.RemoveAll(cache) // hundreds of MB each
			b.StartTimer()
		}
	})
}
