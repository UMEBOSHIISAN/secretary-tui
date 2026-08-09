# Security policy

## Supported version

Security fixes are evaluated against the current `main`. No long-term support window is implied for earlier tags.

## Report privately

Use a [GitHub Security Advisory](https://github.com/UMEBOSHIISAN/secretary-tui/security/advisories/new) to report a
suspected vulnerability privately.

**Do not open a public issue** containing credentials, private paths, personal data, real operational records, or
anything that could expose another person's environment.

Include only what is needed to reproduce:

- affected commit;
- operating system and Go version;
- the exact command line, including any `--governance` or `--dump` arguments;
- a fictional input file that triggers the behaviour;
- expected and actual behaviour.

This program is normally pointed at real operational files. **Sanitize before sending.** If a fictional file cannot
reproduce the issue, describe the shape of the input instead of pasting it.

## What counts as a vulnerability here

The program is read-only, so the interesting failures are disclosure and integrity failures:

- any write, process launch, or network request that the operator did not explicitly request;
- reading a path outside the ones named on the command line;
- rendering untrusted file content in a way that escapes the terminal, injects control sequences, or corrupts the
  display of other panels;
- displaying stale data as current, or a missing source as a zero value;
- leaking a value into `--dump` output that was not visible on screen.

The last two are not cosmetic. A dashboard that shows a confident wrong number is worse than one that shows nothing.

## Response boundary

A report is evidence for review. It is not permission to access another system, rotate credentials, publish details, or
deploy a fix. Maintainers will reproduce with sanitized local data, scope the impact, add a regression test, and
coordinate disclosure separately.
