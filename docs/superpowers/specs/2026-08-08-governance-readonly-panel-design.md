# Secretary TUI — Read-only Governance Panel Design

Date: 2026-08-08
Status: human-approved design

## Goal

Add an optional `AI governance` panel that reads one explicitly selected local
JSON file produced by Workflow Governance Model or Mothership Router. The
feature makes reviewed state visible without granting approval, execution,
discovery, notification, or write authority.

## Command-line contract

```sh
secretary-tui --governance /absolute/or/relative/result.json
secretary-tui --dump --governance ./result.json
```

If `--governance` is omitted, Secretary TUI behaves exactly as it does today.
The application never searches for governance files automatically and does not
read an environment variable for this feature.

## Supported documents

### WGM public handoff 1.0

Recognized by `schema_version`, `task_id`, `capability`, and `risk`. The panel
renders only those public metadata fields plus `token_budget` and the number of
`evidence_references`.

### Mothership Router dry-run manifest

Recognized by `status`, `authority_effect`, and `execution_effect`. The panel
renders status, recommended alias when present, and reasons. A document is
accepted only when both effect fields are exactly `false`.

The TUI does not infer that either document is approved, executed, fresh, or
trustworthy. It labels the view as a local snapshot.

## Safety boundary

- Read one user-specified file with `os.ReadFile`; never write to it.
- Reject files larger than 1 MiB.
- Reject malformed JSON and unsupported document shapes.
- Recursively reject secret-bearing key names: `password`, `secret`,
  `api_key`, `access_token`, `refresh_token`, `credential`, and `private_key`.
- Ignore unknown non-secret fields when rendering; never print raw JSON.
- Never execute a command based on document content.
- Never treat a recommendation, approval record, or status label as authority.
- Existing `llm-seat.sh list` behavior remains unchanged and is not connected
  to the governance panel.

## Architecture

Create `governance.go` as an isolated reader/decoder/renderer model:

```text
explicit --governance path
        |
        v
size check -> JSON decode -> recursive secret-key scan
        |
        v
WGM handoff parser OR Router manifest parser
        |
        v
small display struct -> read-only TUI panel
```

`main.go` owns flag parsing, refresh scheduling, and panel placement. The
governance reader returns a display struct and an error; it has no Bubble Tea
dependency and can be unit tested independently.

## Refresh and errors

The selected file is read during initial refresh, every existing ten-second
refresh, and manual `r` refresh. Missing, oversized, malformed, secret-bearing,
or unsupported documents do not crash the application. The panel shows an
unavailable state while the existing warnings line contains a concise error.
No retry beyond the normal visible refresh cycle is added.

## User interface

Place `AI governance` below the worker panel. Display at most:

- source type: `WGM handoff` or `Router manifest`
- task ID and risk for WGM
- status and recommended alias for Router
- `authority: none` and `execution: none`
- local snapshot label

The panel must remain useful in `--dump` output and must not print source file
contents or secret values.

## Tests

Add standard-library Go tests covering:

1. valid WGM handoff rendering;
2. valid Router dry-run manifest rendering;
3. rejection when either effect is `true`;
4. recursive secret-key rejection;
5. malformed and oversized files;
6. omitted flag preserving the existing model;
7. `--dump --governance` producing the new panel without writes.

Run `go test ./...`, `go vet ./...`, and `go build ./...`. Existing GitHub
Actions will exercise build and vet; tests will be added to the same workflow
only if a separate Execution Gate approval is granted for that workflow file.

## Non-goals

- No provider API, credential loading, file discovery, directory watcher, or
  network request.
- No approval button, execution button, alert, notification, retry, fallback,
  scheduler, or background worker.
- No changes to WGM, Mothership Router, Mothership, xops, RAG, or llm-seat.
