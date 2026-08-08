# Secretary TUI Plans

Created: 2026-08-08

## Phase 1: Read-only governance observation

| Task | Content | DoD | Depends | Status |
| --- | --- | --- | --- | --- |
| 1.1 | Add a bounded reader for WGM handoffs and Router manifests. | Valid public documents render; malformed, oversized, secret-bearing, or effectful documents fail closed in tests. | - | cc:完了 |
| 1.2 | Add optional `--governance` refresh and TUI panel. | Omitted flag preserves current UI; explicit file renders authority-free snapshot in tests and dump mode. | 1.1 | cc:完了 |
| 1.3 | Document compatibility and add tests to CI. | `go test`, `go vet`, `go build`, privacy scan, and GitHub Actions succeed. | 1.1, 1.2 | cc:完了 [c0e3b09] |
