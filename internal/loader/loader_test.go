package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeOverlay(t *testing.T) {
	user := filepath.Join(t.TempDir(), "user.json")
	if err := os.WriteFile(user, []byte(`{"Replace":{"/a.go":"/b.go"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, goflags := range []string{"-mod=mod -overlay=" + user + " -trimpath", "-mod=mod --overlay " + user + " -trimpath"} {
		kept, data, err := mergeOverlay(goflags, "/v/main.go", "/tmp/main.go")
		if err != nil {
			t.Fatal(err)
		}
		if kept != "-mod=mod -trimpath" {
			t.Errorf("GOFLAGS kept = %q", kept)
		}
		var got struct{ Replace map[string]string }
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Replace["/a.go"] != "/b.go" || got.Replace["/v/main.go"] != "/tmp/main.go" {
			t.Errorf("overlay = %v", got.Replace)
		}
	}
	if _, _, err := mergeOverlay("-overlay", "/v", "/r"); err == nil {
		t.Error("want an error for -overlay without a file")
	}
}

func TestWithoutEnv(t *testing.T) {
	got := withoutEnv([]string{"A=1", "GOFLAGS=x", "pwd=/y", "B=2"}, "GOFLAGS", "PWD")
	if len(got) != 2 || got[0] != "A=1" || got[1] != "B=2" {
		t.Errorf("got %v", got)
	}
}
