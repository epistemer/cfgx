package settings_test

import (
	"os"
	"strings"
	"testing"

	"github.com/epistemer/genx/genxtest"
)

// TestGenerated fails when genx check would: cfgx's files against
// the schema (cfgx gen -check), and zz_settings_i18n.go against the labels.
func TestGenerated(t *testing.T) { genxtest.AssertGenerated(t) }

// TestSameDeclarationsAsCfgx is roadmap M11.2's check: the
// zz_settings_i18n.go that genx writes with i18n's generator declares what
// the one cfgx's App.I18n() wrote did (kept in testdata). Only comments
// differ.
func TestSameDeclarationsAsCfgx(t *testing.T) {
	got, err := os.ReadFile("zz_settings_i18n.go")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/cfgx_zz_settings_i18n.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	if g, w := declarations(got), declarations(want); g != w {
		t.Errorf("declarations differ\ngot:\n%s\nwant:\n%s", g, w)
	}
}

// declarations drops comments and blank lines.
func declarations(b []byte) string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if s := strings.TrimSpace(line); s != "" && !strings.HasPrefix(s, "//") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
