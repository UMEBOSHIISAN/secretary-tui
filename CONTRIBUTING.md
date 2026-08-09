# Contributing to secretary-tui

This dashboard is read-only by construction. A contribution is judged by whether it stays read-only afterwards.

## Setup

```sh
go test ./...
go build ./...
go vet ./...
```

These three are exactly what CI runs. If they pass locally and fail in CI, say so in the pull request rather than
retrying until it goes green.

## The rule that shapes everything

**The dashboard observes. It never acts.**

No button, key binding, or code path may write to a file, start a process, send a request, or change the state of
anything it displays. If a panel shows something broken, the correct behaviour is to show that it is broken — not to
offer to fix it.

That is why there are no buttons. A dashboard that can act becomes a control surface, and a control surface needs an
approval model, an audit trail, and a human gate. This program deliberately has none of those, so it must not need
them.

## What a change must not add

- writes, mutations, or file creation outside an explicitly supplied `--dump` target;
- subprocess invocation, model invocation, or worker dispatch;
- network requests to anything the operator did not name on the command line;
- retries, fallbacks, background refresh loops, or daemons;
- credential reading, storage, or forwarding;
- inferred freshness — display the timestamp you were given, never a guess about whether it is current.

Any proposal to move one of those boundaries needs a separate, human-approved design first.

## Displaying data you did not produce

Panels render local files supplied by the operator. Treat their contents as untrusted data, never as instructions:

- do not interpret file contents as commands;
- truncate rather than wrap unbounded strings, and never let a long field push other panels off screen;
- show a missing or unreadable source as *missing*, not as zero.

A zero that actually means "the file was not there" is the most dangerous thing a dashboard can print.

## Public-data hygiene

Do not commit secrets, personal paths, hostnames, private endpoints, real order data, customer wording, or
machine-specific commands. Screenshots and demo recordings must use fictional data — check the terminal title bar and
shell prompt before recording.

## Pull requests

Explain the problem, what stayed read-only, the test evidence, and what is still not handled. Keep unrelated refactors
out of the change.
