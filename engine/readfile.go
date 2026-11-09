package engine

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/epistemer/cfgx"
)

// envValue is the env layer's candidate for key k under the name env: the
// variable itself or, for a FileEnv key, env_FILE naming a file. Both set
// is a problem, reported if this layer wins. nil means neither is set.
func envValue[V any](k *Key[V], env string, lookup func(string) (string, bool)) *candidate {
	raw, ok := lookup(env)
	if !k.FileEnv {
		if !ok {
			return nil
		}
		return &candidate{raw: raw, origin: Origin{LayerEnv, env}}
	}
	// An empty env_FILE names no file, so it counts as not set: that lets
	// a deployment's env_FILE be overridden with env.
	path, fok := lookup(env + "_FILE")
	fok = fok && path != ""
	switch {
	case ok && fok:
		o := Origin{LayerEnv, env}
		return &candidate{origin: o, bad: &cfgx.Problem{Key: k.Path,
			Message: fmt.Sprintf("%s and %s_FILE are both set; set one", env, env), Source: o.String()}}
	case fok:
		return &candidate{raw: path, origin: Origin{LayerEnv, env + "_FILE"}, read: true, trim: true}
	case ok:
		return &candidate{raw: raw, origin: Origin{LayerEnv, env}}
	}
	return nil
}

// resolve is the path c names. A relative path from the config file is
// relative to the file's directory (A11); others, to the working directory.
func (l *loader[V]) resolve(c *candidate) string {
	if c.origin.Layer == LayerFile && relative(c.raw) {
		return filepath.Join(filepath.Dir(l.file.path), c.raw)
	}
	return c.raw
}

// relative reports whether p is relative to a directory. On Windows, a path
// rooted without a drive (\x) or on a drive without a root (D:x) isn't:
// joining it to a directory would change what it means.
func relative(p string) bool {
	return !filepath.IsAbs(p) && filepath.VolumeName(p) == "" && (p == "" || !os.IsPathSeparator(p[0]))
}

// readFile returns the contents of the file c names. It must be a regular
// file, so a FIFO or device can't hang loading, of at most k.MaxFileSize
// bytes. Errors name the path, never the contents.
func (l *loader[V]) readFile(k *Key[V], c *candidate) (string, error) {
	path := l.resolve(c)
	b, err := readRegular(path, k.MaxFileSize)
	if err != nil {
		return "", err
	}
	s := string(b)
	if c.trim {
		if t, ok := strings.CutSuffix(s, "\r\n"); ok {
			s = t
		} else {
			s = strings.TrimSuffix(s, "\n")
		}
	}
	return s, nil
}

func readRegular(path string, limit int64) ([]byte, error) {
	// Stat first: opening a FIFO for reading blocks until a writer comes.
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fileErr(path, err)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s isn't a regular file", path)
	}
	if fi.Size() > limit {
		return nil, fmt.Errorf("%s is larger than %s", path, FormatByteSize(limit))
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fileErr(path, err)
	}
	defer f.Close()
	// The file may have grown since Stat.
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fileErr(path, err)
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s is larger than %s", path, FormatByteSize(limit))
	}
	return b, nil
}

// fileErr words err from reading path without repeating the path twice.
func fileErr(path string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s doesn't exist", path)
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	return fmt.Errorf("can't read %s: %v", path, err)
}
