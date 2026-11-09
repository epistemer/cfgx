package settings_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/epistemer/cfgx"

	"zp/internal/settings"
)

// LoadX loads a command's configuration without cobra (M6.6): env from
// WithEnv, flags and positional arguments from WithArgs, the file from
// WithFile.
func TestLoadMigrate(t *testing.T) {
	ctx := context.Background()
	env := map[string]string{"ZP_DATABASE_NAME": "zp", "ZP_DATABASE_PASSWORD": "pw"}

	m, err := settings.LoadMigrate(ctx, cfgx.WithEnv(env), cfgx.WithArgs("--steps", "3", "--down", "extra"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Steps() != 3 || !m.Down() || m.DBName() != "zp" || m.DBPassword().Reveal() != "pw" || !slices.Equal(m.Args(), []string{"extra"}) {
		t.Errorf("got steps=%d down=%t name=%s args=%q", m.Steps(), m.Down(), m.DBName(), m.Args())
	}

	// A config file, as --config would name it; env still wins over it.
	file := filepath.Join(t.TempDir(), "zp.yaml")
	if err := os.WriteFile(file, []byte("database:\n  name: from-file\n  host: db.internal\n  password: pw\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err = settings.LoadMigrate(ctx, cfgx.WithEnv(map[string]string{"ZP_DATABASE_NAME": "from-env"}), cfgx.WithFile(file))
	if err != nil {
		t.Fatal(err)
	}
	if m.DBHost() != "db.internal" || m.DBName() != "from-env" || len(m.Args()) != 0 {
		t.Errorf("got host=%s name=%s args=%q", m.DBHost(), m.DBName(), m.Args())
	}

	// Problems come back as an error, every one of them.
	_, err = settings.LoadMigrate(ctx, cfgx.WithEnv(nil), cfgx.WithArgs("--steps", "-1"))
	for _, want := range []string{"database.name: is required", "database.password: is required", "steps: must be at least 0 (flag --steps)"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error %v; want %q", err, want)
		}
	}
	if _, err := settings.LoadMigrate(ctx, cfgx.WithEnv(env), cfgx.WithArgs("--nope")); err == nil || !strings.Contains(err.Error(), "unknown flag: --nope") {
		t.Errorf("unknown flag: %v", err)
	}
	// A command's scope holds without cobra too: a key Migrate doesn't use
	// isn't a flag.
	if _, err := settings.LoadMigrate(ctx, cfgx.WithEnv(env), cfgx.WithArgs("--smtp-host", "x")); err == nil {
		t.Error("--smtp-host should be unknown to migrate")
	}
}
