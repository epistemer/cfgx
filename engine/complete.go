package engine

import (
	"strings"

	"github.com/spf13/cobra"
)

// Completion is how a flag's value completes in the shell (Key.Complete).
type Completion struct {
	Exts []string                         // file names with these extensions
	Dirs bool                             // directory names
	Func func(toComplete string) []string // a schema's function
}

// bindCompletions registers the shell completion of the flags of the keys
// at idx on cmd: a key's Complete, or an enum's values.
func bindCompletions[V any](cmd *cobra.Command, keys []Key[V], idx []int) {
	for _, i := range idx {
		k := &keys[i]
		if k.Flag == "" {
			continue
		}
		if fn := completion(k); fn != nil {
			_ = cmd.RegisterFlagCompletionFunc(k.Flag, fn)
		}
	}
}

type compFunc = func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

func completion[V any](k *Key[V]) compFunc {
	c := k.Complete
	switch {
	case c != nil && c.Func != nil:
		f := c.Func
		return func(_ *cobra.Command, _ []string, typed string) ([]string, cobra.ShellCompDirective) {
			return f(typed), cobra.ShellCompDirectiveNoFileComp
		}
	case c != nil && c.Dirs:
		return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveFilterDirs
		}
	case c != nil && len(c.Exts) > 0:
		return fileExts(c.Exts)
	case c != nil:
		return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveDefault
		}
	case len(k.Enum) > 0:
		values := k.Enum
		return func(_ *cobra.Command, _ []string, typed string) ([]string, cobra.ShellCompDirective) {
			var out []string
			for _, v := range values {
				if strings.HasPrefix(v, typed) {
					out = append(out, v)
				}
			}
			return out, cobra.ShellCompDirectiveNoFileComp
		}
	}
	return nil
}

// fileExts completes file names with the extensions exts.
func fileExts(exts []string) compFunc {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return exts, cobra.ShellCompDirectiveFilterFileExt
	}
}
