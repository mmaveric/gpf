# Contributing

## Setup

The module declares Go 1.22 as its minimum. Current local evidence and pending
compatibility checks are recorded in [RELEASE.md](RELEASE.md#compatibility-evidence);
the declaration alone is not a tested support matrix.

From an existing repository checkout, run the small example before making
changes:

```sh
go run ./examples/quickstart
```

The project does not document a public clone URL here; use the checkout/access
instructions provided by its owner.

Start with the [quickstart guide](examples/quickstart/README.md) for typed errors,
capacity, ownership, CPU-affinity fallback, and cooperative shutdown.

## Verification

From the repository root, run the test suite. Use a fresh run when recording
acceptance evidence rather than treating cached results as new execution:

```sh
go test ./...
go test -count=1 ./...
```

For concurrent queue, worker, ingress, metrics, or lifecycle changes, also run
the relevant packages with the race detector; a full check is:

```sh
go test -race ./...
```

Record the toolchain, OS/architecture, command, result, and whether the target
was a clean commit or a working copy. Compilation/cross-compilation is not a
runtime test, and a race run on one platform is not coverage of every platform.
These are local verification commands, not a claim that CI runs them.

Go's `./...` traversal **skips `testdata` directories**. Verify the checked-in
fixture generator explicitly; a passing full-module suite alone does not cover it:

```sh
go test -count=1 ./examples/testdata/generate
GOTOOLCHAIN=go1.22.12 go test -count=1 ./examples/testdata/generate
go test -race -count=1 ./examples/testdata/generate
go vet ./examples/testdata/generate
```

These correctness checks do not require the large JSON files. For optional
fixture generation, explicit output paths, and refusal to overwrite existing
files, see [examples/testdata/README.md](examples/testdata/README.md).

Generated node and graph contracts are committed. After changing their
generators or inputs, regenerate and verify them with:

```sh
go generate ./...
go test ./internal/genarity ./node ./graph
```

Inspect the generated diff and include intended generated changes with their
generator/input changes. Stability tests should detect output drift.

## Performance checks

The framework benchmark matrix can be run with:

```sh
go test -run '^$' -bench . -benchmem ./internal/benchtest ./graph ./queue
```

Keep setup, snapshots, and teardown outside steady-state measurements. Keep
tests that invoke `testing.AllocsPerRun` serial; do not add `t.Parallel` to their
measurement path. Preserve zero-allocation gates rather than weakening them to
pass a newer toolchain. Throughput and latency are non-blocking comparison
signals, and zero allocations do not rule out other performance regressions.
See the [performance contract](spec/05-planning-performance.md) and use the
example READMEs for their workload-specific measurement commands. A full-module
benchmark run also includes example file loaders, which need optional generated
fixtures even though ordinary correctness tests do not. Run application benchmark
comparisons serially; distinguish file-loading/full-workflow from compute-only
boundaries. Historical profile reports are not current performance baselines.
Use the [Showcase](examples/showcase/README.md#reproducible-benchmark-boundaries)
and [standalone](examples/standalone-showcase/README.md#benchmarks-without-overlays)
instructions for fixture generation and explicit `-showcase-data` selection.
Example-only flags must not be passed globally to unrelated test packages.

## Changes

Keep changes consistent with the typed API and ownership contracts documented
in the [specifications](spec/README.md) and package documentation. Include tests
for behavior changes, update documentation, and add a user-visible changelog
entry when appropriate. Prefer keyed literals for exported configuration
structs; newly added fields can break callers using unkeyed literals.

Do not include generated benchmark profiles, input fixtures, or local worklogs
unless the change explicitly calls for them. The readiness journal under
`.scratch/project-readiness/` is local, not release content. Preserve unrelated
working-copy changes when preparing a patch.

Submit changes as a pull request with a concise description of the behavior
changed and the checks run. The repository does not currently specify a
particular pull-request template or commit-message convention.

Licensing, API stability, and publication remain owner decisions; see
[release preparation](RELEASE.md). Updating documentation or passing checks does
not authorize a release, tag, or push.
