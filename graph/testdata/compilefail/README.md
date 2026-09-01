# Typed Connection Compile-Fail Fixture

The fixtures are excluded from normal builds with `//go:build compilefail`.
Run `go test -tags=compilefail ./graph/testdata/compilefail` to verify that
`Connect` rejects an `OutPort[int]` connected to an `InPort[string]` and an
`ErrorPort[int]`; `ConnectError` rejects an `ErrorPort[int]` connected to an
`InPort[string]`.
