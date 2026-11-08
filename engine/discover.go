package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/epistemer/cfgx/internal/node"
)

// configFile is a config file that was found and parsed.
type configFile struct {
	path string
	root *node.Node
}

// extensions are tried in this order in each search path (A11).
var extensions = []struct {
	ext    string
	format string
}{{".yaml", "yaml"}, {".yml", "yaml"}, {".json", "json"}}

func (a *App[V]) allows(format string) bool {
	return len(a.Formats) == 0 || slices.Contains(a.Formats, format)
}

// findConfig returns the path of the config file to read, or "" if there is
// none. explicit is the --config value or, failing that, the env var's; it
// must exist. Otherwise the search paths are tried in order: a path whose
// variables are unset or empty is skipped, and two matching files in one
// directory are an error.
func findConfig[V any](a *App[V], explicit, explicitFrom string, env func(string) (string, bool)) (string, error) {
	if explicit != "" {
		if err := checkFormat(a, explicit); err != nil {
			return "", fmt.Errorf("the config file %s (from %s) %v", explicit, explicitFrom, err)
		}
		info, err := os.Stat(explicit)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("the config file %s (from %s) doesn't exist", explicit, explicitFrom)
			}
			return "", fmt.Errorf("the config file %s (from %s): %v", explicit, explicitFrom, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("the config file %s (from %s) isn't a regular file", explicit, explicitFrom)
		}
		return explicit, nil
	}
	for _, sp := range a.SearchPaths {
		dir, ok := expand(sp, env)
		if !ok {
			continue
		}
		var found []string
		for _, e := range extensions {
			if !a.allows(e.format) {
				continue
			}
			p := filepath.Join(dir, a.ConfigFile+e.ext)
			info, err := os.Stat(p)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) || isNotDir(dir) {
					continue
				}
				return "", fmt.Errorf("looking for the config file: %v", err)
			}
			if info.IsDir() {
				continue
			}
			found = append(found, p)
		}
		switch len(found) {
		case 0:
			continue
		case 1:
			return found[0], nil
		default:
			return "", fmt.Errorf("found more than one config file: %s; keep one", strings.Join(found, " and "))
		}
	}
	return "", nil
}

// isNotDir reports whether dir can't be a directory to look in: it doesn't
// exist, can't be reached, or is a file (which gives ENOTDIR, not ENOENT).
func isNotDir(dir string) bool {
	info, err := os.Stat(dir)
	return err != nil || !info.IsDir()
}

// checkFormat checks that path's extension names a format the app accepts.
// Extensions are compared ignoring case, since Windows and macOS file names
// are case-insensitive.
func checkFormat[V any](a *App[V], path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range extensions {
		if ext == e.ext {
			if !a.allows(e.format) {
				return fmt.Errorf("is %s, which this app doesn't accept; use %s", strings.ToUpper(e.format), a.accepted())
			}
			return nil
		}
	}
	return fmt.Errorf("has an unknown format; use %s", a.accepted())
}

func (a *App[V]) accepted() string {
	var exts []string
	for _, e := range extensions {
		if a.allows(e.format) {
			exts = append(exts, e.ext)
		}
	}
	switch len(exts) {
	case 1:
		return exts[0]
	case 2:
		return exts[0] + " or " + exts[1]
	}
	return strings.Join(exts[:len(exts)-1], ", ") + " or " + exts[len(exts)-1]
}

// expand expands $VAR and ${VAR} in a search path. It reports false if any
// variable is unset or empty, so the path is skipped. $HOME comes from
// os.UserHomeDir, which also works where HOME isn't set (Windows).
func expand(path string, env func(string) (string, bool)) (string, bool) {
	ok := true
	out := os.Expand(path, func(name string) string {
		var v string
		if name == "HOME" {
			h, err := os.UserHomeDir()
			if err == nil {
				v = h
			}
		} else {
			v, _ = env(name)
		}
		if v == "" {
			ok = false
		}
		return v
	})
	return out, ok
}

// readConfig reads and parses the config file at path.
func readConfig(path string) (*configFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the config file: %v", err)
	}
	f, ok := node.FormatOf(strings.ToLower(path))
	if !ok {
		f = node.YAML
	}
	root, err := node.Parse(path, data, f)
	if err != nil {
		return nil, fmt.Errorf("the config file is invalid: %v", err)
	}
	return &configFile{path: path, root: root}, nil
}
