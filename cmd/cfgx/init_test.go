package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/epistemer/cfgx/internal/golden"
)

// The skeleton's files, rendered for example.com/hello.
func TestSkeletonGolden(t *testing.T) {
	d, err := newInitData("example.com/hello")
	if err != nil {
		t.Fatal(err)
	}
	files, err := render(d)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".gitignore", "README.md", "internal/logging/logging.go", "internal/settings/schema/schema.go",
		"internal/settings/settings.go", "main.go", "serve.go"}
	if len(files) != len(want) {
		t.Errorf("%d files; want %v", len(files), want)
	}
	for _, p := range want {
		b, ok := files[p]
		if !ok {
			t.Errorf("no %s", p)
			continue
		}
		name := strings.ReplaceAll(strings.TrimPrefix(p, "."), "/", "_") + ".golden"
		golden.Assert(t, filepath.Join("testdata", "init", name), b)
	}
}

func TestInitNames(t *testing.T) {
	for module, want := range map[string]initData{
		"example.com/hello":       {Bin: "hello", Name: "hello", EnvPrefix: "HELLO"},
		"github.com/acme/my-app":  {Bin: "my-app", Name: "my-app", EnvPrefix: "MY_APP"},
		"example.com/tool/v2":     {Bin: "tool", Name: "tool", EnvPrefix: "TOOL"},
		"hello":                   {Bin: "hello", Name: "hello", EnvPrefix: "HELLO"},
		"example.com/2fa":         {Bin: "2fa", Name: "app-2fa", EnvPrefix: "APP_2FA"},
		"example.com/My__Weird.x": {Bin: "My__Weird.x", Name: "my-weird-x", EnvPrefix: "MY_WEIRD_X"},
	} {
		d, err := newInitData(module)
		want.Module = module
		if err != nil || d != want {
			t.Errorf("%s: %+v, %v; want %+v", module, d, err, want)
		}
	}
	for _, bad := range []string{"", "/abs", "a b", "trailing/", `example.com/foo"bar`, "example.com//v2", "example.com/a\nb"} {
		if _, err := newInitData(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// init refuses to overwrite files, and then writes nothing.
func TestInitRefuses(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("main.go", []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := initCmd(context.Background(), []string{"example.com/hello"}, &out, &errb); code != 1 ||
		!strings.Contains(errb.String(), "refusing to overwrite main.go") {
		t.Errorf("exit %d: %s", code, errb.String())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("%d entries in the directory; want only main.go", len(entries))
	}
	if code := initCmd(context.Background(), nil, &out, &errb); code != 2 {
		t.Errorf("no module path: exit %d", code)
	}
}

// A failed step removes what init made, so it can run again.
func TestInitUndo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("notes.txt", []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "") // no go command: the first step fails
	var out, errb bytes.Buffer
	if code := initCmd(context.Background(), []string{"example.com/hello"}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "go mod init") {
		t.Errorf("exit %d: %s", code, errb.String())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "notes.txt" {
		t.Errorf("left %v", entries)
	}
}
