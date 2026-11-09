package mailer_test

import (
	"testing"

	"github.com/epistemer/cfgx"

	"zp/internal/mailer"
	"zp/internal/settings"
)

// The library test from interface.md §5.4: settings.Test gives every
// default, reads no files, and takes changes as plain fields.
func TestDescribe(t *testing.T) {
	m := mailer.New(settings.Test(func(v *settings.Values) {
		v.SMTP.Host = "localhost"
		v.SMTP.Password = cfgx.NewSecret("x")
		v.Auth.EmailFrom = "no-reply@zp.test"
	}))
	got := m.Describe("ops@zp.test")
	want := "from no-reply@zp.test to ops@zp.test via localhost:587 (auth PLAIN, tls false)"
	if got != want {
		t.Errorf("Describe = %q, want %q", got, want)
	}
}
