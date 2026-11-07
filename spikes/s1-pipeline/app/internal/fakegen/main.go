// Command fakegen stands in for `cfgx gen` in spike S1. It writes a temporary
// main package that blank-imports the schema, runs it with `go run`, and reads
// the IR from its stdout. -place selects where the temporary main lives.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const mainSrc = `package main

import (
	"os"

	"example.com/app/internal/fakedsl"
	_ %q
)

func main() {
	if err := fakedsl.Export(os.Stdout); err != nil {
		panic(err)
	}
}
`

func main() {
	place := flag.String("place", "overlay", "overlay | overlay-file | underscore | dot | dot-file | plain")
	flag.Parse()
	schemaPkg := flag.Arg(0)

	dirOut, err := exec.Command("go", "list", "-f", "{{.Dir}}", schemaPkg).Output()
	if err != nil {
		fail(*place, "go list", err, nil)
	}
	dir := strings.TrimSpace(string(dirOut))
	before := entries(dir)

	start := time.Now()
	out, stderr, err := run(*place, dir, fmt.Sprintf(mainSrc, schemaPkg))
	elapsed := time.Since(start)
	if err != nil {
		fail(*place, "go run", err, stderr)
	}

	var defs []map[string]any
	if err := json.Unmarshal(out, &defs); err != nil {
		fail(*place, "decode IR", err, out)
	}
	untouched := entries(dir) == before
	fmt.Printf("place=%-12s ok=true elapsed=%-6s defs=%d tree_untouched_after=%v\n",
		*place, elapsed.Round(time.Millisecond), len(defs), untouched)
}

// run writes the temporary main as the placement requires and runs it.
func run(place, dir, src string) (stdout, stderr []byte, err error) {
	var args []string
	switch place {
	case "overlay", "overlay-file":
		real, err := os.CreateTemp("", "cfgxgen-*.go")
		if err != nil {
			return nil, nil, err
		}
		defer os.Remove(real.Name())
		if _, err := real.WriteString(src); err != nil {
			return nil, nil, err
		}
		real.Close()
		virtualDir := filepath.Join(dir, "cfgxgen_virtual")
		virtual := filepath.Join(virtualDir, "main.go")
		ov, err := json.Marshal(map[string]map[string]string{"Replace": {virtual: real.Name()}})
		if err != nil {
			return nil, nil, err
		}
		ovFile, err := os.CreateTemp("", "cfgxgen-overlay-*.json")
		if err != nil {
			return nil, nil, err
		}
		defer os.Remove(ovFile.Name())
		ovFile.Write(ov)
		ovFile.Close()
		target := "./cfgxgen_virtual"
		if place == "overlay-file" {
			target = "./cfgxgen_virtual/main.go"
		}
		args = []string{"run", "-overlay=" + ovFile.Name(), target}
	default:
		name := map[string]string{"underscore": "_cfgxgen", "dot": ".cfgxgen", "dot-file": ".cfgxgen", "plain": "cfgxgen_tmp"}[place]
		if name == "" {
			return nil, nil, fmt.Errorf("unknown -place %q", place)
		}
		tmp, err := os.MkdirTemp(dir, name)
		if err != nil {
			return nil, nil, err
		}
		defer os.RemoveAll(tmp)
		if err := os.WriteFile(filepath.Join(tmp, "main.go"), []byte(src), 0o644); err != nil {
			return nil, nil, err
		}
		target := "./" + filepath.Base(tmp)
		if place == "dot-file" {
			target += "/main.go"
		}
		args = []string{"run", target}
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outBuf, &errBuf
	err = cmd.Run()
	return outBuf.Bytes(), errBuf.Bytes(), err
}

// entries lists a directory, to detect whether the run left anything behind.
func entries(dir string) string {
	des, _ := os.ReadDir(dir)
	var names []string
	for _, de := range des {
		names = append(names, de.Name())
	}
	return strings.Join(names, ",")
}

func fail(place, step string, err error, detail []byte) {
	fmt.Printf("place=%-12s ok=false step=%s err=%v\n", place, step, err)
	if len(detail) > 0 {
		fmt.Printf("    %s\n", strings.ReplaceAll(strings.TrimSpace(string(detail)), "\n", "\n    "))
	}
	os.Exit(1)
}
