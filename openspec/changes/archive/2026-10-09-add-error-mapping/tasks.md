## 1. Typed Contracts
- [x] 1.1 Confirm `fix-moved-input-ownership` is complete; verify existing typed-error delivery tests before adding mapping contracts.
- [x] 1.2 Define fixed numeric ErrorEvent and ErrorMapper[E]; verify heterogeneous E examples compile without boxing and conversion returns the expected payload.
- [x] 1.3 Add immutable typed mapper configuration and dormant generated hooks; verify registration/Build/no-logger Start do not invoke mapping.
## 2. Verification and Documentation
- [x] 2.1 Regenerate arity bindings and add callback-contract tests; verify generator stability and concrete mapping allocation gates with nonallocating callbacks.
- [x] 2.2 Document payload limits and logging activation dependency, reconcile current mapping deferral deltas, and run `openspec validate add-error-mapping --strict` plus affected Go tests.
