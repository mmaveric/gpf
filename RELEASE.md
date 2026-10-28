# Release preparation and support evidence

The selected target is **v0.1.0 experimental**, with no pre-1.0 API-stability
promise. This is **not a completed release, release announcement, or
authorization to publish**. MIT was selected and added in `LICENSE`, with the
copyright notice `Copyright (c) 2026 Altynbek Sharipov`. The owner still needs to
ensure they have rights to distribute all included materials. Choose and review
the exact commit before depending on it.

## Compatibility evidence

The 2026-10-27 validation used an independent snapshot of the completed working
copy based on `d49ad54`, **not a frozen release commit**. Its 123 Go/module files
matched the live source before and after verification. User JSON, profiles, SVG,
and local worklogs were excluded; subsequent documentation edits are separate.
Do not treat this evidence as a support promise for every Go version, OS,
architecture, cpuset, or possible execution.

Source-set SHA-256 (sorted paths, each `path + NUL + contents + NUL`):
`21d808191709ed5aa7495683ba3c5d890119f4f65f0219260e3c19199e51a343`.

| Scope | Recorded evidence | Qualification |
| --- | --- | --- |
| Declared minimum | `go.mod` declares Go 1.22 | Go 1.22.0 itself was not tested; see the Go 1.22.12 result below. |
| Fresh suites | `go test -count=1 ./...` passed for all 12 discovered packages on Go 1.26.8 and Go 1.22.12, linux/amd64 | Same completed source snapshot; tests ran before any large JSON was generated. Not evidence for Go 1.22.0. |
| Race / vet | `go test -race -count=1 ./...` and `go vet ./...` passed on Go 1.26.8 | Covers executed test paths, not all possible executions or other platforms. |
| Fixture generator | Explicit package tests on both Go versions, plus race and vet on Go 1.26.8, passed | `./...` skips `testdata`; run `./examples/testdata/generate` separately. |
| Generated drift | `go generate ./...` passed on Go 1.26.8; all snapshot Go files were unchanged | Zero changed, added, or deleted Go files; generation occurred only in the independent copy. |
| No-cgo builds | `CGO_ENABLED=0 go build ./...` passed on Go 1.26.8 for linux/amd64, windows/amd64, and darwin/amd64 | Compilation only; no Windows/macOS runtime verification. |
| Quickstart | Exact documented stdout and four-node DOT with two data edges and one error edge verified | Snapshot run; host affinity success does not guarantee affinity elsewhere. |
| Fixture / application runs | Newly generated external valid/mixed million-record files matched documented hashes; both apps matched totals on both files | Showcase reported zero queue/pool drops and pools in use after drain. Existing user inputs were not required or overwritten. |
| Full-workflow benchmarks | One serial iteration per app with the same new mixed fixture and no overlay passed | Smoke only, not a performance comparison or baseline; load-only, compute-only, and TSC benchmarks were not rerun here. |

The validator also confirmed all nine existing user JSON/profile/SVG artifacts
retained their hashes. This snapshot verification does not replace independent
contract review, resolve the conformance findings below, or authorize publication.

The runtime attempts affinity on Linux and otherwise has a locked-thread
fallback; rejected Linux requests also fall back. This design is not proof of
runtime compatibility on all OS/architecture pairs. CPU IDs are explicit plan
inputs, not discovered host availability. Affinity success does not prove NUMA
memory locality. NUMA metadata and allocator descriptors remain inactive.

No CI matrix is established by this document. Expanding CI is outside this
preparation's scope; record local checks honestly rather than implying automated
coverage. Benchmark results are workload/toolchain/hardware-specific and do not
set a portable throughput promise.

## Owner decisions before publication

- [x] Select MIT and record the copyright notice in `LICENSE`.
- [ ] Confirm rights to distribute all repository code and included materials;
  repository access is not a substitute for a grant.
- [x] Select the documentation target: **v0.1.0 experimental**, not a release.
- [ ] Decide whether to publish at all and select the exact candidate commit.
  Local inspection found no tags; this is a local fact, not
  proof that no remote tag or hosted release exists.
- [ ] Define the intended minimum Go version, tested Go versions, and target
  OS/architecture set. Distinguish compile-only checks from runtime/race tests.
- [x] Mark the target experimental with no pre-1.0 API-stability promise;
  breaking API changes remain possible. Do not infer stability from generated
  APIs or a v0.x number.
- [x] Review the direct `github.com/loov/hrtime v1.0.3` license: its checked-in
  license is MIT, compatible with GPF's MIT license. Preserve its notice when
  distributing copies or substantial portions; include a third-party notice for
  binary distributions as prudent practice.

## Candidate verification checklist

These are gates to review, not assertions that the release is ready:

- [ ] Freeze and identify the candidate; preserve unrelated user changes and
  distinguish working-copy results from clean-commit results.
- [x] Rewrite the five topic specs to remove duplicated historical stage
  restrictions and document source-verified limitations. This documentation work
  does **not** establish that every intended guarantee is implemented or tested.
- [ ] Complete the [conformance review gates](#conformance-review-gates) below:
  resolve each finding with appropriate verification, or obtain explicit owner
  acceptance of the retained limitation before publication.
- [x] Verify quickstart stdout/DOT and both applications' documented fixture
  totals in the completed source snapshot. Portable affinity fallback remains a
  contract, not an outcome observed on every host.
- [x] Record fresh dual-Go suites, race/vet, zero generated drift, explicit
  generator-package checks, and no-cgo cross-builds as scoped above. Keep
  allocation assertions serial; the snapshot does not freeze a release commit.
- [ ] Complete the independent contract review and decide any follow-up changes.
  If source changes, repeat the affected checks before selecting the candidate.
- [ ] Review known limitations: cooperative cancellation, no panic recovery,
  unsupported concurrent lifecycle calls, fixed parallelism, topology exclusions,
  inactive NUMA behavior, and no general performance or delivery guarantee.
- [ ] Finalize the [Unreleased changelog](CHANGELOG.md) from completed,
  user-visible work. Do not invent historical versions or release dates.
- [ ] Check the intended publication contents and exclude local worklogs,
  generated profiles, and incidental fixtures unless deliberately approved.
- [ ] Obtain explicit authorization for any commit, tag, release publication,
  or push. Passing the checklist does not grant that authorization.

### Conformance review gates

The rewritten specs distinguish current behavior from intended guarantees.
Passing existing tests, vet, or bounded race checks does not close these findings
or establish final-candidate readiness. For each item, record either a verified
resolution or explicit owner acceptance of the retained limitation, and align
public contracts and release notes with that decision. Acceptance is not a claim
that the intended guarantee has been implemented. No runtime fix is authorized
by this checklist.

- [x] **Pre-worker startup rollback — accepted limitation:** factory/normalization
  failures before workers exist currently Close owned lifecycles on the startup
  caller, not a locked owner worker. The owner accepts this for `v0.1.0
  experimental`; no universal owner-worker rollback guarantee is made for that
  phase. See the [startup conformance check](spec/02-execution-lifecycle.md#startup-conformance-check).
- [x] **Metadata identity and ordering — accepted limitation:** `GroupNUMA`
  exposes physical `GroupID`, replica, and placement; expanded metric/affinity
  rows follow physical-plan order, not globally ascending logical IDs. The owner
  accepts this representation for `v0.1.0 experimental`; no separate logical-
  group ID or logical-order snapshot guarantee is made. See [definition
  inspection](spec/01-typed-graph.md#build-and-immutable-definitions) and
  [numeric snapshots](spec/04-errors-observability.md#numeric-runtime-snapshots).
- [x] **NUMA placement and observation — deferred milestone:** NUMA annotations
  remain descriptive only; memory placement is inactive. The owner defers both
  activation and complete propagation of annotations to pool snapshots (including
  ordinary-edge and broadcast/shard scratch pools) until after the first release.
  See [inactive NUMA metadata](spec/05-planning-performance.md#numa-metadata-and-inactive-allocators).
- [x] **Benchmark dimensions versus logical work:** remote/DAG operations now
  process and check 64 logical elements; typed-error cases are named for one error
  event per failure. Package tests and one-iteration benchmark smoke passed. This
  verifies the workload shape, not a performance baseline; repeat checks on the
  frozen candidate. See [benchmark conformance](spec/05-planning-performance.md#benchmark-and-allocation-conformance).

These are bounded source-review findings, not an exhaustive conformance audit.
The completed-source snapshot passed the checks above, but independent contract
review and owner decisions remain open. Neither the spec rewrite nor those
results declare v0.1.0 ready for publication.
