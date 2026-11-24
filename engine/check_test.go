package engine

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/epistemer/cfgx"
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

// unlucky is an app's own error type, as a Validate function returns it.
type unlucky struct{ n int }

func (u unlucky) Error() string { return "is unlucky" }

// A Validate function's error reaches Problem.Err unchanged, at startup
// and from the default; cfgx's own constraints leave Err nil (A15).
func TestProblemErr(t *testing.T) {
	load := func(def int, env map[string]string) cfgx.Errors {
		t.Helper()
		k := Key[chkVals]{
			Path: "n", Env: "CFGXT_N",
			Set:   func(v *chkVals, r string) error { return ParseInt(r, &v.N) },
			IsSet: func(v *chkVals) bool { return v.N != 0 },
			Check: func(v *chkVals) error { return AtMost(v.N, 10, "10") },
			Validate: func(v *chkVals) error {
				if v.N == 7 {
					return unlucky{v.N}
				}
				return nil
			},
		}
		if def != 0 {
			k.Default = func(v *chkVals) { v.N = def }
		}
		_, err := Load(&App[chkVals]{EnvPrefix: "CFGXT", Keys: []Key[chkVals]{k}}, nil, []int{0}, Options{
			LookupEnv: func(s string) (string, bool) { v, ok := env[s]; return v, ok },
			Environ:   func() []string { return nil },
		})
		var errs cfgx.Errors
		if !errors.As(err, &errs) || len(errs) != 1 {
			t.Fatalf("%v; want one cfgx.Errors problem", err)
		}
		return errs
	}
	for _, p := range []cfgx.Problem{
		load(0, map[string]string{"CFGXT_N": "7"})[0],
		load(7, nil)[0],
	} {
		var u unlucky
		if !errors.As(p.Err, &u) || u.n != 7 || p.Message != "is unlucky" || p.Key != "n" {
			t.Errorf("%+v; want Err unlucky{7}, Message its text", p)
		}
	}
	if p := load(0, map[string]string{"CFGXT_N": "11"})[0]; p.Err != nil || p.Message != "must be at most 10" {
		t.Errorf("a constraint: %+v; want Err nil", p)
	}
	if p := load(0, map[string]string{"CFGXT_N": "x"})[0]; p.Err != nil {
		t.Errorf("a parse error: %+v; want Err nil", p)
	}
	// 70 fails both: the constraint runs first, so the problem is cfgx's.
	if p := load(0, map[string]string{"CFGXT_N": "70"})[0]; p.Err != nil || p.Source != "env CFGXT_N" {
		t.Errorf("both fail: %+v; want the constraint's problem, Err nil", p)
	}
}

// A key with a Validate function and no constraints is checked too.
func TestValidateOnly(t *testing.T) {
	k := Key[chkVals]{
		Path: "n", Env: "CFGXT_N",
		Set:      func(v *chkVals, r string) error { return ParseInt(r, &v.N) },
		IsSet:    func(v *chkVals) bool { return v.N != 0 },
		Validate: func(v *chkVals) error { return unlucky{v.N} },
	}
	_, err := Load(&App[chkVals]{EnvPrefix: "CFGXT", Keys: []Key[chkVals]{k}}, nil, []int{0}, Options{
		LookupEnv: func(s string) (string, bool) { return "3", s == "CFGXT_N" },
		Environ:   func() []string { return nil },
	})
	var errs cfgx.Errors
	if !errors.As(err, &errs) || len(errs) != 1 || errs[0].Err != (unlucky{3}) {
		t.Errorf("%v; want Err unlucky{3}", err)
	}
}
