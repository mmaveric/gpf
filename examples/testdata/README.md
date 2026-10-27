# Deterministic order fixtures

The optional local fixtures contain one million orders each. Both applications
load the valid `examples/data.json` by default; `orders-5pct-invalid.json` contains
the same records with exactly 50,000 invalid orders (5%). Ordinary correctness
tests do not require either large JSON file.

## Generate and run

Run these commands from the repository root. Generate only missing files: the
[generator](generate/main.go) requires explicit output paths and refuses to
overwrite existing files or symlinks. If the datasets already exist locally,
skip generation and use them unchanged.

```sh
go run ./examples/testdata/generate -valid-out examples/data.json
go run ./examples/testdata/generate -mixed-out examples/testdata/orders-5pct-invalid.json
go run ./examples/showcase -data examples/testdata/orders-5pct-invalid.json
go run ./examples/standalone-showcase -data examples/testdata/orders-5pct-invalid.json
```

`-valid-out` and `-mixed-out` may be supplied together or independently; at least
one is required. Output directories must already exist. An I/O failure can leave
a partial new file, and generating two files is not an atomic transaction.
Existing files are never replaced.

Each app's `-data PATH` selects that exact file, with no default-path fallback.
Use an absolute path when launching a prebuilt binary from another directory.
No dataset replacement, symlink setup, environment override or source overlay is
needed. For timing options and file-loading/full-workflow benchmark commands,
see the [Showcase README](../showcase/README.md) and
[standalone README](../standalone-showcase/README.md). Both full/load benchmark
packages accept `-args -showcase-data=/absolute/path/to/orders.json`.

The narrow local ignore rules exclude only the generated JSON paths, not this
README or `generate/` source. Generating fixtures does not modify tracked source.

## Record formulas and mixed validity

For zero-based record `i`, the valid generator uses `id=i+1`,
`customerId=i%10000+1`, `units=i%10+1`, and `unitCents=50+(i*37)%9951`.

Invalid orders are spread throughout the file: one per 20-record block. Using
zero-based block number `g`, the modified record is at index `20*g + g%20`.
This rotates the position within each block instead of always invalidating the
same units/customer pattern. Every 64-record ingress batch contains both valid
and invalid orders.

Each modified order has exactly one field set to zero, cycling through:

| Field | Invalid orders |
|---|---:|
| `id` | 16,667 |
| `units` | 16,667 |
| `unitCents` | 16,666 |

The record count, order and all other fields match the original dataset.

## Expected mixed-input result

Expected summary for both Showcase and standalone:

```text
orders submitted=1000000 processed=950000 rejected=50000 audited=950000 units=5225000 gross_cents=26253957313 partitions=317300,315400,317300
```

Showcase should additionally report zero queue/pool drops, zero logged handler
errors, and zero pools in use after draining. Invalid records use the validator's
ordinary typed output, not handler failures.

## Encoding and verification

Each file starts with `[\n`, contains compact JSON records separated by `,\n`,
and ends with `\n]\n`. Fields appear in the order `id`, `customerId`, `units`,
`unitCents`. Generator tests pin the complete million-record SHA-256 hashes:

- Original: `ff0e21f2f39a79ed3f18f805cd516adf66d8d8f997fb8deb69e7e383791a8460`
- Mixed: `e77a1fcf1b062915b644308422ca53601e83550d547c63de4cd453ad783a15f0`

```sh
go test -count=1 ./examples/testdata/generate
```

The generator reproduces the existing local fixture bytes; it does not rewrite
them. Historical runs used a temporary working directory with a dataset symlink
before the apps supported `-data`; that setup is no longer required.
