# Typed Connection Compile-Fail Fixture

`connect_mismatched.go` is excluded from normal builds with `//go:build ignore`.
Remove that build constraint and run `go test ./graph/testdata/compilefail` to
verify that `Connect` rejects an `OutPort[int]` connected to an `InPort[string]`.
