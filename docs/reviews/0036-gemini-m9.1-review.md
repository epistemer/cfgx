# Review 0036: Gemini review of M9.1 (benchmarks against viper)

| | |
|---|---|
| Date | 2026-11-10 |
| Roadmap item | M9.1, cross-model review before merge |
| Reviewer | Gemini 3.8 Flash (Medium), through agy's `adversarial_review` (session `e349c3ea-79d7-4a9f-bbdb-0de3554f1041`), about 255k tokens, on the diff without the generated code |
| Scope | The bench module (`examples/zp/bench`), its README, the README's numbers, the CI step |
| Result | 10 findings: 6 accepted and fixed, 4 rejected. The fixes changed viper's numbers by less than 10%; the ratios hold. |

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F01 | Major | The viper side built a `strings.Replacer` per key and formatted every key name on each run, which a real app wouldn't. | Accepted: it sets the env prefix and replacer once and takes key names from the table. |
| F02 | Major | The viper side loaded 40 keys; zp's Serve loads 39 (`storage.bolt` isn't in it). | Accepted: `storage.bolt` is gone from the viper side. |
| F03 | Major | `-short` skipped `BenchmarkGenerate` in CI. | Accepted: CI runs the warm generate benchmark once; the cold one (20 s) stays local. |
| F04 | Major | The checks didn't prove env vars and secret files were loaded. | Accepted: both sides check an env value, the env secret and a JWT key file. |
| F05 | Major | "It is roughly the code that cfgx generates" overstated the likeness; many viper apps use `Unmarshal`. | Accepted: the README says what the viper side does and leaves out, and that `Unmarshal` isn't measured. |
| F06 | Minor | The alias was registered after reading the file. | Accepted: aliases are registered first. |
| F07 | Minor | The viper side doesn't apply `RelevantIf`. | Rejected as a change: noted in the README as work cfgx does that the viper side leaves out. |
| F08 | Minor | `cmd.Dir` is relative. | Rejected: `go test` runs in the package directory, and `b.Chdir` restores it. |
| F09 | Minor | `GOCACHE` may be duplicated in the env, and cold caches pile up on disk. | Half accepted: each cold cache is removed after its run. A duplicate is harmless: `os/exec` keeps the last value. |
| F10 | Minor | The CI step runs only on ubuntu with Go 1.27. | Rejected: the step guards the benchmarks from rotting; the code they load is tested on the whole matrix. |
