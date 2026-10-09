## 1. Inactive Metadata
- [x] 1.1 Confirm automatic CPU binding is complete; verify physical plan inspection tests before metadata extension.
- [x] 1.2 Add optional NUMA annotations with explicit unspecified semantics; verify zero defaults, invalid IDs, and copied pool/group/queue metadata.
- [x] 1.3 Define typed dormant allocator descriptors; verify compile-time types and zero descriptor calls during registration, Build, and sequential Starts.
## 2. Compatibility and Verification
- [x] 2.1 Verify existing CPU/queue/allocation behavior remains unchanged with annotations; test source compatibility examples and document keyed configuration expectations.
- [x] 2.2 Document inactive status and run affected Go tests and `openspec validate add-numa-allocation-metadata --strict`; verify no platform topology or NUMA allocation is activated.
