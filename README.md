# cfgx

Config-first, type-safe configuration for Go command-line apps built on
[cobra](https://github.com/spf13/cobra). You declare every setting once, in a Go
schema, and `go generate` turns it into typed getters and interfaces, flags, env
vars, config-file keys, docs and example files.

> **Status: pre-alpha.** Nothing works yet. The design lives in [docs/](docs/):
>
> - [interface.md](docs/interface.md): what you write and what you get back (locked)
> - [implementation.md](docs/implementation.md): how it's built
> - [roadmap.md](docs/roadmap.md): the plan, milestone by milestone

## License

MIT. See [LICENSE](LICENSE).
