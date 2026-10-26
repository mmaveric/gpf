# Contributing

## Development

The module requires Go 1.22 or newer. From the repository root, run the test
suite with:

```sh
go test ./...
```

Generated node and graph contracts are committed. After changing their
generators or inputs, regenerate and verify them with:

```sh
go generate ./...
go test ./internal/genarity ./node
```

The benchmark matrix can be run with:

```sh
go test -run '^$' -bench . -benchmem ./...
```

## Changes

Keep changes consistent with the typed API and ownership contracts documented
in the README and package documentation. Include tests for behavior changes and
update documentation when public behavior changes. Do not include generated
benchmark profiles or other local artifacts in a change.

Submit changes as a pull request with a concise description of the behavior
changed and the checks run. The repository does not currently specify a
particular pull-request template or commit-message convention.
