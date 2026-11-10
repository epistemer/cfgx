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

## Performance

Measured against viper on the same inputs, on a 2015 laptop (Intel Core
i5-5300U) with Go 1.27.3. [examples/zp/bench](examples/zp/bench) has the code,
what each benchmark does, and how to run them.

| | cfgx | viper |
|---|---|---|
| Load zp's `serve` configuration, 39 keys | 0.31 ms, 869 allocs | 0.56 ms, 1145 allocs |
| Load 100 keys | 0.46 ms, 1701 allocs | 1.85 ms, 2606 allocs |

`go generate` (`cfgx gen`) on zp's schema takes 1.4 s with a warm Go build
cache. With a cold cache it takes 20 s, about what a cold `go build` of the
app takes (18 s).

## License

MIT. See [LICENSE](LICENSE).
