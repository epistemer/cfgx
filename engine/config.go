package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/epistemer/cfgx/internal/opts"
	"github.com/epistemer/cfgx/internal/storage"
)

// ConfigCommand returns the config subcommand (interface §8): init, print,
// explain, validate, schema and env. Each covers every key of the app but
// command arguments, or, given a command line such as "migrate" or "db
// migrate", that command's keys. Values come from defaults, the config file
// and env vars only: never a command's flags (A3, A9).
func ConfigCommand[V any](a *App[V]) *cobra.Command {
	c := &configCmd[V]{a: a}
	root := &cobra.Command{
		Use:   "config",
		Short: "Show, check and set up this program's configuration",
	}
	scoped := func(use, short string, run func(cmd *cobra.Command, idx []int, withStore bool) error) *cobra.Command {
		return &cobra.Command{
			Use:   use + " [command]",
			Short: short + "; for one command's keys, name it",
			RunE: func(cmd *cobra.Command, args []string) error {
				cmd.SilenceUsage = true
				idx, profile, err := c.scope(cmd, args)
				if err != nil {
					return err
				}
				// The store's values are shown for every key, or for a
				// command that has the store.
				withStore := c.a.OpenStore != nil && (profile == "" || c.a.StoreProfiles[profile])
				return run(cmd, idx, withStore)
			},
		}
	}
	root.AddCommand(
		scoped("print", "Print the effective configuration as YAML, secrets masked", c.print),
		scoped("explain", "List every key with its value and where it came from", c.explain),
		scoped("validate", "Load and check the configuration", c.validate),
		scoped("env", "List the environment variables this program reads", c.env),
	)
	if a.ConfigFile != "" {
		var force bool
		var output string
		initCmd := &cobra.Command{
			Use:   "init",
			Short: "Write the example configuration as " + a.ConfigFile + ".yaml",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				cmd.SilenceUsage = true
				return c.init(cmd, output, force)
			},
		}
		initCmd.Flags().BoolVar(&force, "force", false, "overwrite the file if it exists")
		initCmd.Flags().StringVarP(&output, "output", "o", a.ConfigFile+".yaml", "the file to write")
		root.AddCommand(initCmd, &cobra.Command{
			Use:   "schema",
			Short: "Print the config file's JSON Schema",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				_, err := io.WriteString(cmd.OutOrStdout(), a.JSONSchema)
				return err
			},
		})
	}
	return root
}

type configCmd[V any] struct{ a *App[V] }

// scope is the keys a subcommand covers: every key but command arguments,
// or the keys of the command args names.
func (c *configCmd[V]) scope(cmd *cobra.Command, args []string) ([]int, string, error) {
	var idx []int
	profile := ""
	if len(args) == 0 {
		for i := range c.a.Keys {
			idx = append(idx, i)
		}
	} else {
		target, rest, err := cmd.Root().Find(args)
		if err == nil && len(rest) == 0 && target != cmd.Root() {
			profile = target.Annotations[profileAnnotation]
		}
		keys, ok := c.a.Profiles[profile]
		if profile == "" || !ok {
			return nil, "", fmt.Errorf("%q isn't a command with a configuration", strings.Join(args, " "))
		}
		idx = keys
	}
	// Command arguments are flags only, and config reads no flags (A9).
	out := idx[:0:0]
	for _, i := range idx {
		if !c.a.Keys[i].FlagOnly {
			out = append(out, i)
		}
	}
	return out, profile, nil
}

// load loads the keys at idx from defaults, the file and env: the only
// flag it reads is --config. With the store, stored values are loaded from
// it opened read-only.
func (c *configCmd[V]) load(cmd *cobra.Command, idx []int, partial, withStore bool) (*Result[V], error) {
	fs := pflag.NewFlagSet("config", pflag.ContinueOnError)
	if f := cmd.Flags().Lookup("config"); f != nil {
		fs.AddFlag(f)
	}
	o := Options{Partial: partial}
	if withStore {
		o.Store = StoreReadOnly
	}
	res, err := Load(c.a, fs, idx, o)
	if res.Store != nil {
		res.Store.Close()
	}
	var ro *opts.Root
	if r, ok := roots.Load(cmd.Root()); ok {
		ro = r.(*opts.Root)
	}
	logWarnings(cmd.Context(), loggerFor(ro, cmd.ErrOrStderr()), res.Warnings)
	if err != nil {
		cmd.SilenceErrors = true
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", cmd.Root().Name(), err)
	}
	return res, err
}

// show is a key's value as text for a table: strings as they are, the rest
// as JSON.
func show(v any) string {
	if s, ok := v.(string); ok {
		return cell(s)
	}
	return jsonText(v)
}

// cell quotes s if it would break a table row.
func cell(s string) string {
	if strings.ContainsAny(s, "\t\n\r") {
		return strconv.Quote(s)
	}
	return s
}

func jsonText(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

// yamlNode is one level of the printed YAML: a map, or a key's value.
type yamlNode struct {
	name  string
	kids  []*yamlNode
	value any
	leaf  bool
}

func (n *yamlNode) child(name string) *yamlNode {
	for _, k := range n.kids {
		if k.name == name && !k.leaf {
			return k
		}
	}
	c := &yamlNode{name: name}
	n.kids = append(n.kids, c)
	return c
}

func (n *yamlNode) write(b *strings.Builder, indent string) {
	for _, k := range n.kids {
		if k.leaf {
			fmt.Fprintf(b, "%s%s: %s\n", indent, k.name, jsonText(k.value))
			continue
		}
		fmt.Fprintf(b, "%s%s:\n", indent, k.name)
		k.write(b, indent+"  ")
	}
}

// shown is the keys print, explain and env list: the scope's, then the
// RelevantIf conditions they depend on, which are loaded with them.
func (c *configCmd[V]) shown(idx []int) []int {
	return append(append([]int(nil), idx...), conditions(c.a, idx)...)
}

// print writes the effective configuration as YAML: every key, at its
// config file path, with JSON values, which YAML reads too.
func (c *configCmd[V]) print(cmd *cobra.Command, idx []int, withStore bool) error {
	res, err := c.load(cmd, idx, true, withStore)
	if err != nil {
		return err
	}
	root := &yamlNode{}
	for _, i := range c.shown(idx) {
		k := &c.a.Keys[i]
		segs := strings.Split(k.Path, ".")
		n := root
		for _, s := range segs[:len(segs)-1] {
			n = n.child(s)
		}
		n.kids = append(n.kids, &yamlNode{name: segs[len(segs)-1], value: k.Show(res.Values), leaf: true})
	}
	var b strings.Builder
	root.write(&b, "")
	_, err = io.WriteString(cmd.OutOrStdout(), b.String())
	return err
}

// explain lists every key with its value and where the value came from.
func (c *configCmd[V]) explain(cmd *cobra.Command, idx []int, withStore bool) error {
	res, err := c.load(cmd, idx, true, withStore)
	if err != nil {
		return err
	}
	tw := newTable(cmd.OutOrStdout())
	fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE\tDEFAULT")
	for _, i := range c.shown(idx) {
		k := &c.a.Keys[i]
		def := k.DefaultText
		if k.DefaultFrom != "" {
			def = k.DefaultFrom + "()"
		}
		source := "unset"
		if o := res.Origins[i]; len(o) > 0 {
			last := o[len(o)-1]
			source = last.String()
			switch {
			case last.Layer == LayerDefault && k.DefaultFrom != "":
				source = "default " + k.DefaultFrom + "()"
			case last.Layer == LayerStore:
				source = storeSource(res.Records[i], o[:len(o)-1])
			}
		}
		if why, ok := res.Invalid[i]; ok {
			source += " (stored value ignored: " + why + ")"
		}
		if res.Irrelevant[i] {
			source += " (not relevant)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", k.Path, show(k.Show(res.Values)), cell(source), cell(def))
	}
	return tw.Flush()
}

// validate loads the keys as a command would, Required and rules included.
func (c *configCmd[V]) validate(cmd *cobra.Command, idx []int, withStore bool) error {
	if _, err := c.load(cmd, idx, false, withStore); err != nil {
		return err
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), "The configuration is valid.")
	return err
}

// env lists the env vars the keys read, and whether each is set now.
func (c *configCmd[V]) env(cmd *cobra.Command, idx []int, _ bool) error {
	tw := newTable(cmd.OutOrStdout())
	fmt.Fprintln(tw, "VARIABLE\tKEY\tSET\tDESCRIPTION")
	row := func(name, key, desc string) {
		set := ""
		if _, ok := os.LookupEnv(name); ok {
			set = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", name, key, set, desc)
	}
	if c.a.ConfigFile != "" {
		row(c.a.configEnv(), "", "the config file to read")
	}
	for _, i := range c.shown(idx) {
		k := &c.a.Keys[i]
		if k.Env == "" {
			continue
		}
		row(k.Env, k.Path, k.Desc)
		if k.FileEnv {
			row(k.Env+"_FILE", k.Path, "a file holding "+k.Path)
		}
	}
	return tw.Flush()
}

// init writes the example configuration, which comments every key out, so
// the file changes nothing until a line is uncommented.
func (c *configCmd[V]) init(cmd *cobra.Command, path string, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%s exists; pass --force to overwrite it", path)
	}
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, c.a.ExampleConfig); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s.\n", path)
	return err
}

// textTable aligns tab-separated columns, without trailing spaces where a row's
// last cells are empty.
type textTable struct {
	*tabwriter.Writer
	buf bytes.Buffer
	out io.Writer
}

func newTable(out io.Writer) *textTable {
	t := &textTable{out: out}
	t.Writer = tabwriter.NewWriter(&t.buf, 0, 4, 2, ' ', 0)
	return t
}

func (t *textTable) Flush() error {
	if err := t.Writer.Flush(); err != nil {
		return err
	}
	var b strings.Builder
	for _, l := range strings.SplitAfter(t.buf.String(), "\n") {
		trimmed := strings.TrimRight(l, " \n")
		if trimmed == "" && l == "" {
			continue
		}
		b.WriteString(trimmed)
		if strings.HasSuffix(l, "\n") {
			b.WriteString("\n")
		}
	}
	_, err := io.WriteString(t.out, b.String())
	return err
}

// storeSource is explain's source for a stored value: when, who and why,
// and the sources it hides.
//
//	store (2026-11-02, admin@zp: "PCI review"), hides env ZP_AUTH_OTP_LENGTH
func storeSource(r storage.Record, below []Origin) string {
	var parts []string
	if !r.UpdatedAt.IsZero() {
		parts = append(parts, r.UpdatedAt.Format("2006-01-02"))
	}
	switch {
	case r.UpdatedBy != "" && r.Reason != "":
		parts = append(parts, r.UpdatedBy+": "+strconv.Quote(r.Reason))
	case r.UpdatedBy != "":
		parts = append(parts, r.UpdatedBy)
	case r.Reason != "":
		parts = append(parts, strconv.Quote(r.Reason))
	}
	s := "store"
	if len(parts) > 0 {
		s += " (" + strings.Join(parts, ", ") + ")"
	}
	var hidden []string
	for _, o := range below {
		if o.Layer != LayerDefault {
			hidden = append(hidden, o.String())
		}
	}
	if len(hidden) > 0 {
		s += ", hides " + strings.Join(hidden, ", ")
	}
	return s
}
