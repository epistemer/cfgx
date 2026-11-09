package engine

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

type chkVals struct{ N int }

// Check runs on a set value, from any layer, the default included, and
// names the source; an unset value isn't checked.
func TestCheck(t *testing.T) {
	load := func(def string, env map[string]string) error {
		k := Key[chkVals]{
			Path: "n", Env: "CFGXT_N", Desc: "N",
			Set:   func(v *chkVals, r string) error { return ParseInt(r, &v.N) },
			IsSet: func(v *chkVals) bool { return v.N != 0 },
			Check: func(v *chkVals) error {
				if err := AtMost(v.N, 10, "10"); err != nil {
					return err
				}
				if v.N == 7 {
					return errors.New("7 is unlucky")
				}
				return nil
			},
		}
		if def != "" {
			k.Default = func(v *chkVals) { _ = ParseInt(def, &v.N) }
		}
		_, err := Load(&App[chkVals]{EnvPrefix: "CFGXT", Keys: []Key[chkVals]{k}}, nil, []int{0}, Options{
			LookupEnv: func(s string) (string, bool) { v, ok := env[s]; return v, ok },
			Environ:   func() []string { return nil },
		})
		return err
	}
	for _, c := range []struct {
		def  string
		env  map[string]string
		want string
	}{
		{"", nil, ""},
		{"", map[string]string{"CFGXT_N": "0"}, ""},
		{"", map[string]string{"CFGXT_N": "11"}, "n: must be at most 10 (env CFGXT_N)"},
		{"", map[string]string{"CFGXT_N": "7"}, "n: 7 is unlucky (env CFGXT_N)"},
		{"12", nil, "n: must be at most 10 (default)"},
	} {
		err := load(c.def, c.env)
		if (c.want == "") != (err == nil) || err != nil && !strings.Contains(err.Error(), c.want) {
			t.Errorf("default %q, env %v: %v; want %q", c.def, c.env, err, c.want)
		}
	}
}

func TestConstraintMessages(t *testing.T) {
	if err := Length("héllo", 1, 4); err == nil || err.Error() != "must be 1 to 4 characters long, not 5" {
		t.Errorf("Length: %v", err)
	}
	if err := Length("ab", 3, 3); err == nil || err.Error() != "must be 3 characters long, not 2" {
		t.Errorf("Length exact: %v", err)
	}
	re := regexp.MustCompile(`^(?:[a-z]+)$`)
	if Match("abc", re, "[a-z]+") != nil || Match("abc1", re, "[a-z]+") == nil {
		t.Error("Match isn't a whole-value match")
	}
	if AtLeast(1.5, 1.5, "1.5") != nil || AtLeast(-1, 0, "0") == nil {
		t.Error("AtLeast")
	}
}
