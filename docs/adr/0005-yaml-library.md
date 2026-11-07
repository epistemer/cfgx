# ADR 0005: YAML and JSON File Parser Library

| | |
|---|---|
| Status | Proposed |
| Date | 2026-11-07 |
| Roadmap item | M1.4 |
| Deciders | Claude (owner) decides; Gemini proposed |

## Context

`cfgx` parses configuration files (both YAML and JSON) into its own intermediate node tree (`internal/node`: `{Kind, Scalar, Map, List, Pos}`) rather than unmarshaling into `map[string]any` (`docs/implementation.md` §3.2, §3.4; ADR 0001 D1, D7). Every key and value must carry a 1-based `line:column` source position (`File:Line:Col`), scalar text must remain in its original form so `cfgx`'s per-key parsers (`strconv`, `time.ParseDuration`, etc.) control type conversion without YAML 1.1/1.2 surprises, duplicate mapping keys must be rejected with source positions, and untrusted or malformed inputs (alias bombs, merge keys, multi-document streams, deep nesting) must be handled safely.

Because JSON is a subset of YAML 1.2, `docs/implementation.md` §3.4 proposes parsing JSON files through the same YAML parser and node builder so there is a single tree walker with `line:column` positions for both formats. Spike S4 (`spikes/s4-yaml`) evaluated the three live candidates against the archived `gopkg.in/yaml.v3` baseline:

- `go.yaml.in/yaml/v4` `v4.0.0` (the YAML organization's actively developed v4 line)
- `go.yaml.in/yaml/v3` `v3.0.6` (the YAML organization's security-fix-only v3 line)
- `github.com/goccy/go-yaml` `v1.19.2`
- `gopkg.in/yaml.v3` `v3.0.1` (baseline only; archived on 2025-04-01)

All reproducible tests and benchmarks live in the standalone module `spikes/s4-yaml` (`module github.com/epistemer/cfgx/spikes/s4-yaml`).

## Options

### Comparison Table

| Criterion | `go.yaml.in/yaml/v4` (`v4.0.0`) | `go.yaml.in/yaml/v3` (`v3.0.6`) | `github.com/goccy/go-yaml` (`v1.19.2`) | `gopkg.in/yaml.v3` (`v3.0.1`, baseline) |
|---|---|---|---|---|
| **1. Node API with positions & raw scalar text** (`TestCriterion1_NodeAPIPositionsAndRawScalars`) | `yaml.Load(data, &doc)` into `yaml.Node`. 1-based `Line` & `Column` on every key and value node. `Node.Value` preserves exact scalar text; `Node.ShortTag()` (`!!float`, `!!int`, `!!str`, etc.) + `Node.Style` (quoted vs plain) cleanly distinguish `1.0`, `"1.0"`, `1`, `0x10`, `"0x10"`, `yes`, `"yes"`. | `yaml.Unmarshal` / `NewDecoder` into `yaml.Node`. Same `Line`, `Column`, `Value`, `ShortTag()`, and `Style` fidelity as `v4`. | `parser.ParseBytes(data, 0)` into `*ast.File`. 1-based `Line` & `Column` on `node.GetToken().Position`; raw text on `Token.Value`. **Bug**: lexer's `token.toNumber` checks `strings.Count(v, ".") == 0` and calls `strconv.ParseUint`, so scientific floats without a dot (`1e10`, `1e-5`) and ints $\ge 2^{64}$ fail number parsing and become `*ast.StringNode` (`!!str`). | Same `yaml.Node` tree, `Line`, `Column`, `Value`, `ShortTag()`, and `Style` as `go.yaml.in/yaml/v3`. |
| **2. Strict duplicate keys (YAML & JSON)** (`TestCriterion2_DuplicateKeys`) | Enabled by default in `Constructor` (`WithUniqueKeys(true)`), **but** loading into `yaml.Node` bypasses `Constructor.checkMappingKeys` and retains duplicate pairs in `MappingNode.Content`. Checking `seen[key.Value]` during `cfgx`'s `MappingNode` walk (5 LoC in `ParseV4`) rejects YAML & JSON duplicates with exact `line:col` for both duplicate (`3:1` / `4:3`) and original (`1:1` / `2:3`) keys. | Same as `v4`: `Constructor` check is bypassed when target is `yaml.Node`; 5-LoC check in `MappingNode` walk reports exact `line:col` for both keys in YAML and JSON. | `parser.ParseBytes` rejects duplicates by default with `[line:col]`. **Bug**: `parser/context.go` tracks seen keys in a single document-wide flat map `p.pathMap[ctx.path]` and wraps keys containing `.` in single quotes without escaping existing `'`, so valid JSON `{"'a'": {"'b'": 1}, "a'.'b": 2}` fails with false-positive `[3:3] mapping key "a'.'b" already defined at [2:11]`. | Same as `go.yaml.in/yaml/v3`. |
| **3. JSON input through YAML parser** (`TestCriterion3_JSONThroughYAML`) | Parses nested objects/arrays, compact JSON (`{"a":1}`), tab indentation, escapes (`é`, `\u00e9`, `\"`, `\\`, `\/`, `\b`, `\f`, `\n`, `\r`, `\t`), ints $> 2^{63}$, floats (`1e10`, `1e-5`), `true`/`false`/`null`, `{}`/`[]`. **Divergences from `encoding/json`**: (1) rejects UTF-16 surrogate escapes (`\uD83D\uDE00`) in favor of raw UTF-8 `"😀"` or `\U0001F600`; (2) tags ints $\ge 2^{64}$ as `!!float` (raw digits preserved in `Node.Value`); (3) accepts YAML superset syntax (comments, trailing commas). | **Fails on valid JSON `\/` escape** (`yaml: line 4: found unknown escape character` in `scannerc.go:2550`, which lacks `case '/':`). Otherwise shares `v4`'s surrogate-pair and YAML-superset divergences. | Accepts `\uD83D\uDE00` and `\/`, **but** (1) misclassifies scientific floats without a decimal point (`1e10`, `1e-5`) and ints $\ge 2^{64}$ as `*ast.StringNode` (`!!str`), and (2) has the false-positive duplicate-key collision on keys with quotes and dots. | **Fails on valid JSON `\/` escape** (`yaml: line 4: found unknown escape character`), same as `go.yaml.in/yaml/v3`. |
| **4. YAML 1.1 vs 1.2 quirks** (`TestCriterion4_YAML11Vs12Quirks`) | Zero silent conversions in `yaml.Node`: `yes`, `no`, `on`, `off`, `y`, `n` have `Tag: "!!str"` and raw `Value`; `0777`, `0o777`, `1_000` have `Tag: "!!int"` and raw `Value` (`"0777"`, `"0o777"`, `"1_000"`); `~` has `Tag: "!!null", Value: "~"`; `.inf` has `Tag: "!!float", Value: ".inf"`; `2026-11-07` has `Tag: "!!timestamp", Value: "2026-11-07"`. | Identical to `v4` in `yaml.Node`. | `yes`, `no`, `on`, `off`, `y`, `n` and `2026-11-07` are `StringNode` (`!!str`); `0777`, `0o777`, `1_000` are `IntegerNode` (`Token.Value` preserves raw string, though `Node.Value` pre-parses `uint64(511)` / `uint64(1000)`); `~` is `NullNode`; `.inf` is `InfinityNode`. | Identical to `v4` in `yaml.Node`. |
| **5. Safety** (`TestCriterion5_Safety`) | **Aliases**: unexpanded `AliasNode` in `yaml.Node` ($O(n)$ memory); `ParseV4` rejects `AliasNode` (or caps expansion count), and `Constructor` rejects billion laughs (`document contains excessive aliasing`); `plugin/loaderlimits` & `plugin/aliasdata` available. **Merge keys**: unquoted `<<` has `Tag: "!!merge"` (rejected by `ParseV4`); quoted `"<<": val` is `!!str`. **Multi-doc**: `yaml.Load` rejects multi-doc and empty streams **by default** (`expected single document, found multiple`). **Deep nesting**: scanner rejects $> 10{,}000$ levels in $< 1\text{ ms}$ (`exceeded max depth of 10000`). | **Aliases / Merge / Nesting**: same scanner/composer protections as `v4`. **Multi-doc**: `yaml.Unmarshal` **silently drops** trailing documents; requires `yaml.NewDecoder` + second `Decode` checking `io.EOF`. | **Aliases**: **no alias expansion limit** when decoding into typed Go slices (`TestCriterion5_Safety/BillionLaughsAliasExpansion` expands $9^6 = 531{,}441$ strings from a 6-line input) or walking AST aliases. **Multi-doc**: `yaml.Unmarshal` silently drops extra docs (`parser.ParseBytes` requires checking `len(f.Docs) == 1`). **Deep nesting**: `parser.ParseBytes` has **no depth limit** ($O(n^2)$ `CreateGroupedTokens` takes ~4.6 s on 10,005 `[` brackets before `Decoder` checks `maxDecodeDepth`). | Same as `go.yaml.in/yaml/v3` (`Unmarshal` silently drops extra documents). |
| **6. Error messages (`line:column`)** (`TestCriterion6_ErrorMessages`) | Syntax and structure errors include `L<line>.C<col>` in `err.Error()` (e.g., `at L2.C4-L3.C1`) **and** unwrap via `errors.As` to `*yaml.LoadError` with 1-based `Mark{Line, Column}`. | **Line only, no column** (`yaml: line 3: ...`). Line-1 scanner errors (e.g. invalid escape on line 1) omit even the line number (`yaml: found unknown escape character`). No structured error fields. | Includes `[line:col]` and annotated source snippet on syntax/structure errors. | Same as `go.yaml.in/yaml/v3`: line only, no column, and no line number on line-1 scanner errors. |
| **7. Maintenance** | **Active primary line**. Latest: `v4.0.0` (2026-11-04) after 9 RCs (`v4.0.0-rc.1` 2025-07-30 to `rc.9` 2026-11-02). 66 open issues (100 open issues+PRs). Maintained by the official [YAML organization](https://github.com/yaml/go-yaml) (Ingy döt Net, Mike Farah, et al.). | **Frozen / security-fixes only**. Latest: `v3.0.6` (2026-10-13; `v3.0.2`–`v3.0.5` in 2025–2026). Maintained in [`yaml/go-yaml` `v3` branch](https://github.com/yaml/go-yaml/tree/v3); README states v3 receives security fixes only. | Latest: `v1.19.2` (2026-01-08; prior `v1.19.0` 2025-11-29, `v1.19.1` 2025-12-17; last push 2026-04-11, ~10 months without release). 274 open issues+PRs. Primarily single maintainer (`@goccy`, spare-time project per [README](https://github.com/goccy/go-yaml)). | **Archived / unmaintained**. Latest: `v3.0.1` (2022-05-27). Repository [`go-yaml/yaml`](https://github.com/go-yaml/yaml) archived read-only on 2025-04-01 with 422 open issues+PRs. |
| **8. Dependency weight & binary size** (`TestCriterion8_BinarySizeAndDeps`) | **0 transitive deps** in `go.mod` / `go.sum`. Stripped binary (`-trimpath -ldflags="-s -w"`): `2,625,696 B` vs `1,593,504 B` baseline (**`+1,032,192 B` / `+1,008.0 KiB`**). | **0 transitive deps** in `go.mod` / `go.sum`. Stripped binary: `2,285,728 B` (**`+692,224 B` / `+676.0 KiB`**). | **0 transitive deps** in `go.mod` / `go.sum`. Stripped binary (`parser` only): `2,105,504 B` (**`+512,000 B` / `+500.0 KiB`**). | `gopkg.in/check.v1` in `go.sum`. Stripped binary: `2,285,728 B` (**`+692,224 B` / `+676.0 KiB`**). |
| **9. Speed (~100-key config -> `cfgx` Node tree)** (`BenchmarkCriterion9_Parse100Keys`) | **YAML**: `399 µs/op` (`115.8 KB/op`, `1,508 allocs/op`)<br>**JSON**: `593 µs/op` (`440.9 KB/op`, `1,343 allocs/op`) | **YAML**: `407 µs/op` (`115.1 KB/op`, `1,475 allocs/op`)<br>**JSON**: `422 µs/op` (`111.6 KB/op`, `1,303 allocs/op`) | **YAML**: `761 µs/op` (`338.7 KB/op`, `5,704 allocs/op`)<br>**JSON**: `670 µs/op` (`388.4 KB/op`, `6,298 allocs/op`) | **YAML**: `364 µs/op` (`115.1 KB/op`, `1,475 allocs/op`)<br>**JSON**: `382 µs/op` (`111.6 KB/op`, `1,303 allocs/op`) |

*(Reference: `encoding/json` unmarshaling the same 100-key JSON into `map[string]any` without source positions takes `121 µs/op`, `19.8 KB/op`, `477 allocs/op`. All four YAML libraries parse a 100-key config in $0.36\text{–}0.76\text{ ms}$, well under 1 ms.)*

### Option Details

#### 1. `go.yaml.in/yaml/v4` (`v4.0.0`) — Recommended
- **Benefits**:
  - **Complete `line:column` fidelity**: Every key and value `yaml.Node` carries 1-based `Line` and `Column`, and unlike `v3`, syntax and scanner errors also carry 1-based `Line` and `Column` both in `err.Error()` (`L2.C4`) and in structured `*yaml.LoadError` (`Mark{Line, Column}`).
  - **Fixes `v3`'s JSON `\/` escape bug**: `internal/libyaml/scanner.go:2345` supports `\/` inside double-quoted strings, so standard JSON with escaped forward slashes parses cleanly.
  - **Rejects multi-document streams by default**: `yaml.Load(data, &doc)` returns `yaml: expected single document, found multiple` out of the box without needing a manual two-call `Decoder` loop.
  - **Raw scalar preservation**: `Node.Value` retains the exact unquoted scalar text (`"1.0"`, `"1"`, `"0x10"`, `"0777"`, `"1_000"`, `"yes"`, `"2026-11-07"`), `Node.ShortTag()` provides the resolved YAML 1.2 schema tag (`yes`/`no`/`on`/`off`/`y`/`n` are `!!str`), and `Node.Style` distinguishes quoted strings from plain scalars.
  - **Active maintenance & zero transitive dependencies**: Maintained by the official YAML organization (`v4.0.0` released 2026-11-04) with zero external module dependencies.
- **Costs**:
  - Adds `+1,008 KiB` to a stripped binary (vs `+676 KiB` for `v3`).
  - Like `v3`, `yaml.Load(data, &docNode)` short-circuits `Constructor.checkMappingKeys` when the target is `yaml.Node`, so `cfgx`'s `MappingNode` walker must check `seen[key.Value]` during tree construction (5 lines of code, which also yields a better error message with `line:column` of both keys).
  - Rejects UTF-16 surrogate-pair escapes (`\uD83D\uDE00`) in JSON strings because YAML 1.2 forbids surrogate code points (`U+D800..U+DFFF`) in `\uXXXX` and uses `\U0001F600` or raw UTF-8 `"😀"`.

#### 2. `go.yaml.in/yaml/v3` (`v3.0.6`)
- **Benefits**: Same `yaml.Node` tree API as `v4`, zero transitive dependencies, slightly smaller binary delta (`+676 KiB`).
- **Costs**:
  - **Rejects valid JSON `\/` escape**: `scannerc.go:2550` fails with `yaml: found unknown escape character` on `"\/"`.
  - **No column numbers in syntax/scanner errors**: Error strings only report `line N` (and omit even the line number on line-1 scanner errors), with no structured error position type.
  - **Silently ignores extra documents in `yaml.Unmarshal`**: Requires `yaml.NewDecoder` + a second `Decode` call checking `io.EOF`.
  - **Maintenance frozen**: Per the upstream README, `v3` receives security fixes only; bug fixes go to `v4`.

#### 3. `github.com/goccy/go-yaml` (`v1.19.2`)
- **Benefits**: Smallest binary delta when importing `parser` only (`+500 KiB`), pretty error formatting with `[line:col]`, accepts UTF-16 surrogate pairs in JSON.
- **Costs**:
  - **False-positive duplicate-key bug**: `parser/context.go` tracks duplicate keys in a single document-wide map keyed by a dot-joined path with unescaped single quotes (`$.'a'.'b'`), causing distinct keys at different depths (e.g., `{"'a'": {"'b'": 1}, "a'.'b": 2}`) to fail with a bogus duplicate key error (`TestCriterion2_DuplicateKeys/GoccyFalsePositiveDuplicateKeyPathCollisionBug`).
  - **Number lexing bug on scientific notation and big ints**: `token.toNumber` checks `strings.Count(v, ".") == 0` and calls `strconv.ParseUint`, so valid JSON/YAML floats like `1e10` and `1e-5` (and integers $\ge 2^{64}$) are misclassified as `*ast.StringNode` (`!!str`) (`TestCriterion3_JSONThroughYAML`).
  - **Safety gaps in AST/Decoder**: `parser.ParseBytes` has no nesting depth limit and exhibits quadratic token grouping (~4.6 s on 10,005 `[` brackets), and `Decoder` has no alias expansion count limit when decoding into typed Go slices (`TestCriterion5_Safety`).
  - **Slower & higher allocation count**: ~1.9x slower on YAML (`761 µs` vs `399 µs`) with ~3.8x more allocations (`5,704` vs `1,508 allocs/op`).

#### 4. `gopkg.in/yaml.v3` (`v3.0.1`, baseline)
- **Status**: Archived on 2025-04-01; shares all `go.yaml.in/yaml/v3` defects (rejects JSON `\/`, no column on syntax errors, `Unmarshal` ignores trailing documents) plus lacks the 2025–2026 security/bug fixes from `go.yaml.in/yaml`. Disqualified.

## Decision

Use **`go.yaml.in/yaml/v4` (`v4.0.0`)** to parse both YAML and JSON configuration files into `cfgx`'s `internal/node` tree via `yaml.Load(data, &doc)`. Keep the single YAML-based parser path for both YAML and JSON files; a separate `encoding/json/jsontext` parser path is not needed.

## Consequences

1. **Updates to `docs/implementation.md` §3.4 (File source & node builder)**:
   - Replace the `gopkg.in/yaml.v3` / `goccy/go-yaml` placeholder with `go.yaml.in/yaml/v4` (`v4.0.0`).
   - Confirm that the **JSON-through-YAML plan holds**: `.yaml`, `.yml`, and `.json` files all go through `yaml.Load(data, &doc)` into `yaml.Node` and the same `internal/node` tree builder.
2. **Implementation requirements for `internal/node` (M3.1) and `source/file` (M5.2)**:
   - **Single-document enforcement**: Call `yaml.Load(data, &doc)` (which rejects multi-document streams and empty streams by default; treat a comment-only/empty file as an empty root map).
   - **Duplicate-key check in the `MappingNode` walker**: Because `yaml.Load(data, &doc)` bypasses `Constructor.checkMappingKeys` when the target is `yaml.Node`, the `MappingNode` loop in `internal/node` must check `seen[keyNode.Value]` as it walks `Content[i], Content[i+1]` pairs and return a `ParseError` at `keyNode.Line:keyNode.Column` referencing the prior definition at `prev.Line:prev.Column`.
   - **Reject aliases and merge keys**: In the node walker, reject `yaml.AliasNode` and unquoted `!!merge` (`<<`) keys with a `ParseError` at their `Line:Column` so config files remain static, self-contained trees immune to alias expansion bombs.
   - **Preserve `Scalar`, `Tag`, and `Quoted` metadata**: Record `Node.Value`, `Node.ShortTag()`, and whether `Node.Style` is quoted (`SingleQuotedStyle | DoubleQuotedStyle | LiteralStyle | FoldedStyle`) so `cfgx`'s per-key value parsers convert raw text according to the schema's target type.
   - **Unwrap `*yaml.LoadError` for syntax errors**: Use `errors.As(err, &loadErr)` on errors returned by `yaml.Load` to extract `loadErr.Mark.Line` and `loadErr.Mark.Column` into `cfgx.ParseError.Position`.
3. **Documented JSON edge-case divergences**:
   - Document in `source/file` that JSON files parsed through the YAML 1.2 loader accept YAML superset syntax (such as `#` comments and trailing commas) and require supplementary Unicode characters ($> \text{U+FFFF}$) to be written as literal UTF-8 (e.g. `"😀"`) or `\U0001F600` rather than UTF-16 surrogate pairs (`\uD83D\uDE00`).
