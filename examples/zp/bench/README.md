# Benchmarks: cfgx and viper

This module compares cfgx with [viper](https://github.com/spf13/viper) on the
same inputs. It is a module of its own so that viper never becomes a
dependency of cfgx or of the zp example.

| Benchmark | What it does |
|-----------|--------------|
| `BenchmarkZpServe` | Loads zp's `serve` configuration: 39 keys from a config file, env vars, flags and defaults. It validates them (required keys, ranges, enums, the email check, the at-most-one rule) and reads the two JWT key files. |
| `BenchmarkKeys100` | Loads 100 keys (ints, strings, durations and bools in ten groups): 30 from a config file, 10 from env vars, 5 from flags, the rest defaults. |
| `BenchmarkGenerate` | Runs `cfgx gen -check` on zp's schema, which is what `go generate` costs. *warm* has the Go build cache filled. *cold* starts every run with an empty build cache, as on a fresh machine or CI runner. The module cache stays, so nothing is downloaded. |

The viper side ([viper_test.go](viper_test.go)) is what an app would write by
hand to get the same result. It sets a default, an env binding and a flag
binding for every key, then reads the config file. Then it reads each value
into a struct with `GetString`, `GetInt` and the like, and checks it the way
the schema does. Its checks are required keys, ranges, enums, the email
address, the at-most-one rule and the secret files. cfgx does a little more
that the viper side leaves out: it records where each value came from and
applies `RelevantIf`. Many viper apps use `Unmarshal` instead of the getters,
which goes through mapstructure and reflection; it isn't measured here.

## Results

Measured on 2026-11-10 with Go 1.27.3 on linux/amd64, an Intel Core i5-5300U
(a 2015 laptop, 2 cores and 4 threads). Load times are the median of six
runs.

| | cfgx | viper |
|---|---|---|
| zp `serve`, 39 keys | **0.31 ms**, 85 KB, 869 allocs | 0.56 ms, 215 KB, 1145 allocs |
| 100 keys | **0.46 ms**, 158 KB, 1701 allocs | 1.85 ms, 970 KB, 2606 allocs |

| `cfgx gen` on zp's schema | Time |
|---|---|
| warm build cache | 1.4 s |
| cold build cache | 20 s |

For scale, `go build` of the zp example takes 18 s with a cold cache and 0.5 s
with a warm one. Most of a cold `cfgx gen` goes to compiling the standard
library, which the build that follows then reuses.

What the numbers mean:

- **Loading.** cfgx loads zp's configuration in a little over half the time
  viper takes, and 100 keys in a quarter of the time, with fewer
  allocations.
  Either way it's well under a millisecond at startup, so speed is not a
  reason to choose between them. These numbers show only that cfgx doesn't
  cost more.
- **Generating.** About 0.85 s of a warm `cfgx gen` is spent in the `go`
  commands it runs. The largest part, 0.4 s, is linking the small program
  that evaluates the schema. The rest is listing packages, which includes
  type-checking the settings package to find the computed methods. The
  remaining 0.6 s is cfgx itself: running the schema, then rendering and
  comparing the files. A Go installed as a snap adds about 0.5 s of launcher
  time to each run (2 s instead of 1.4 s on this machine); the numbers above
  call the Go binary directly.

## Running them

```bash
cd examples/zp/bench
go test -run='^$' -bench='ZpServe|Keys100' -count=6 -benchtime=2s .
go test -run='^$' -bench='Generate/warm' -benchtime=5x .
go test -run='^$' -bench='Generate/cold' -benchtime=2x .
```

`keys100/zz_settings.go` is generated from `keys100/schema` with
`go generate ./keys100`. The schema builds its keys in a loop, in a variable's
initializer. It can't use `init`, which runs after `App`'s initializer has
already captured the (still empty) list of groups.
