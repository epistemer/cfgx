package settings_test

import (
	"path/filepath"
	"testing"
	"time"

	"zp/internal/settings"
	"zp/models"
)

// Test fills in every default, the computed ones included, without the
// JWT key files that serve would read: their values stay empty.
func TestDefaults(t *testing.T) {
	all := settings.Test(nil)
	if all.ServerPort() != 7777 || all.ServerTimeoutRead() != 20*time.Second || all.ServerMaxHeader() != 1<<20 {
		t.Errorf("server defaults: %d %s %d", all.ServerPort(), all.ServerTimeoutRead(), all.ServerMaxHeader())
	}
	if all.SchedulerExecmode() != models.ZpTaskExecutionModeSequential || all.SchedulerWorkers() < 1 {
		t.Errorf("scheduler defaults: %s %d", all.SchedulerExecmode(), all.SchedulerWorkers())
	}
	if loc := all.SchedulerLocation(); loc == nil || loc.String() != "Africa/Cairo" {
		t.Errorf("location = %v", loc)
	}
	// A Path is cleaned for the OS: ssl\cert.pem on Windows.
	if all.AuthJWTPrivate().IsSet() || all.ServerTLSCert() != filepath.FromSlash("ssl/cert.pem") {
		t.Errorf("file keys: the key file must not be read, the path default applies")
	}
	p, err := all.RetryPolicy()
	if err != nil || p.Max != 3 || p.Delay != time.Minute {
		t.Errorf("RetryPolicy = %+v, %v", p, err)
	}
	if all.DSN() != "postgres://postgres:@localhost:5432?sslmode=disable" {
		t.Errorf("DSN = %s", all.DSN())
	}
	// Changes are per call: a second Test starts from the defaults again.
	if settings.Test(func(v *settings.Values) { v.Server.Port = 1 }).ServerPort() != 1 || settings.Test(nil).ServerPort() != 7777 {
		t.Error("edit")
	}
}
