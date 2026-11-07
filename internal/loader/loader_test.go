package loader

import (
	"testing"
)

func TestSplitOverlayFlag(t *testing.T) {
	for _, in := range []string{"-mod=mod -overlay=/o.json -trimpath", "-mod=mod --overlay /o.json -trimpath"} {
		rest, ov, err := splitOverlayFlag(in)
		if err != nil || rest != "-mod=mod -trimpath" || ov != "/o.json" {
			t.Errorf("%q: rest=%q overlay=%q err=%v", in, rest, ov, err)
		}
	}
	if rest, ov, err := splitOverlayFlag("-mod=vendor"); err != nil || rest != "-mod=vendor" || ov != "" {
		t.Errorf("no overlay: rest=%q overlay=%q err=%v", rest, ov, err)
	}
	if _, _, err := splitOverlayFlag("-overlay"); err == nil {
		t.Error("want an error for -overlay without a file")
	}
}

func TestWithoutEnv(t *testing.T) {
	got := withoutEnv([]string{"A=1", "GOFLAGS=x", "pwd=/y", "B=2"}, "GOFLAGS", "PWD")
	if len(got) != 2 || got[0] != "A=1" || got[1] != "B=2" {
		t.Errorf("got %v", got)
	}
}
