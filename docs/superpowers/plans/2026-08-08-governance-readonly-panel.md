# Read-only Governance Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional Secretary TUI panel that safely renders one explicitly selected WGM handoff or Mothership Router dry-run manifest.

**Architecture:** A new `governance.go` unit owns bounded file reading, recursive secret-key rejection, document classification, and conversion to a small display struct. `main.go` only parses `--governance`, refreshes the selected file with the existing visible cycle, and renders the display struct without exposing raw JSON.

**Tech Stack:** Go 1.26, standard-library `encoding/json`, Bubble Tea, Lip Gloss, standard-library Go tests.

## Global Constraints

- Read exactly one user-specified file; never auto-discover governance files.
- Reject files larger than 1 MiB.
- Reject secret-bearing key names recursively.
- Router manifests are displayable only when `authority_effect` and `execution_effect` are both exactly `false`.
- No writes, approvals, notifications, provider calls, retries, fallbacks, schedulers, or execution commands.
- Existing behavior remains unchanged when `--governance` is omitted.

---

### Task 1: Bounded governance document reader

**Files:**
- Create: `governance.go`
- Create: `governance_test.go`

**Interfaces:**
- Produces: `type governanceSnapshot struct { source, taskID, risk, status, alias string; reasons []string; evidenceN int; available bool }`
- Produces: `func readGovernance(path string) (governanceSnapshot, error)`
- Produces: `func governanceLines(governanceSnapshot) []string`

- [ ] **Step 1: Write failing tests for valid WGM and Router documents**

Use `t.TempDir()` and `os.WriteFile` to create one WGM 1.0 handoff and one Router manifest. Assert WGM source/task/risk/evidence count and Router status/alias/reasons while both effects remain false.

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `go test ./...`
Expected: FAIL because `readGovernance` and `governanceSnapshot` do not exist.

- [ ] **Step 3: Implement bounded read and typed classification**

Implement `readGovernance` using `os.Stat`, a `1 << 20` byte limit, `os.ReadFile`, `json.Unmarshal` into `map[string]any`, and two private typed decoder structs. Recognize WGM by `schema_version == "1.0"` plus non-empty task ID/capability/risk. Recognize Router by non-empty status plus boolean effect fields that are both false. Return an unsupported-document error otherwise.

- [ ] **Step 4: Add failing safety tests**

Test malformed JSON, a sparse file larger than 1 MiB, nested `api_key`, Router `authority_effect: true`, and Router `execution_effect: true`. Each must return an error and no available snapshot.

- [ ] **Step 5: Implement recursive secret-key rejection**

Walk every `map[string]any` and `[]any`. Normalize key names with `strings.ToLower` and reject exact keys `password`, `secret`, `api_key`, `access_token`, `refresh_token`, `credential`, and `private_key` before typed decoding. Never include source values in error text.

- [ ] **Step 6: Verify Task 1**

Run: `go test ./...`
Expected: all reader and safety tests PASS.

---

### Task 2: Refresh and read-only panel integration

**Files:**
- Modify: `main.go`
- Modify: `governance_test.go`

**Interfaces:**
- Consumes: `readGovernance(path string) (governanceSnapshot, error)`
- Consumes: `governanceLines(governanceSnapshot) []string`
- Produces: `func parseArgs(args []string) (dump bool, governancePath string, err error)`

- [ ] **Step 1: Write failing argument and omitted-path tests**

Assert `parseArgs([]string{"--dump", "--governance", "result.json"})` returns both values, an empty argument list preserves empty governance path, and a missing flag value returns an error.

- [ ] **Step 2: Implement standard flag parsing without global state**

Use a new `flag.FlagSet` with output set to `io.Discard`; define `--dump` and `--governance`. Reject positional arguments. Keep `main()` responsible for printing argument errors and exiting with code 2.

- [ ] **Step 3: Thread governance state through refresh**

Add `governancePath string` and `governance governanceSnapshot` to `model`, and add the snapshot to `refreshMsg`. Change `refreshCmd(home, governancePath)` to skip governance reads for an empty path and call `readGovernance` exactly once otherwise. Include errors in the existing warnings aggregation.

- [ ] **Step 4: Render the optional panel**

When `governancePath` is non-empty, render `AI governance` below workers with `governanceLines`. Show `(governance snapshot unavailable)` on errors. Always show `authority: none`, `execution: none`, and `local snapshot`; never render raw JSON.

- [ ] **Step 5: Add dump integration test**

Construct a model with a temporary Router manifest, run one refresh, call `View`, and assert the output includes `AI governance`, `approval_required`, `authority: none`, and `execution: none` while excluding the raw registry digest.

- [ ] **Step 6: Verify Task 2**

Run: `gofmt -w main.go governance.go governance_test.go`
Run: `go test ./...`
Expected: PASS and existing no-governance behavior remains covered.

---

### Task 3: Public documentation, CI, and release verification

**Files:**
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `.github/workflows/build.yml`
- Create: `Plans.md`

**Interfaces:**
- Consumes: `--governance <path>` and `--dump --governance <path>`
- Produces: public usage, safety, compatibility, and verification guidance.

- [ ] **Step 1: Document usage and compatibility**

Add a governance panel row, commands for WGM and Router files, supported versions (`WGM 0.2.x handoff 1.0`, `Mothership Router 0.2.x`), explicit no-authority language, file-size limit, secret-key rejection, and error behavior.

- [ ] **Step 2: Add release notes**

Add a `v0.2.0` CHANGELOG entry describing the optional panel, explicit-path design, and fail-closed safety behavior. Do not claim automatic integration or execution.

- [ ] **Step 3: Add unit tests to existing CI**

Insert `- run: go test ./...` before build and vet in `.github/workflows/build.yml`. Do not change permissions, triggers, runners, or action versions.

- [ ] **Step 4: Run complete verification**

Run: `go test ./...`
Run: `go vet ./...`
Run: `go build ./...`
Run a tracked-file scan for private paths, token prefixes, and private-key markers.
Expected: all commands succeed and the scan reports no newly introduced secret or private-path content.

- [ ] **Step 5: Commit and publish**

Commit the scoped implementation, push `main`, tag `v0.2.0`, create the GitHub Release, and wait for the `build` workflow to pass. Never force-push or move an existing tag.
