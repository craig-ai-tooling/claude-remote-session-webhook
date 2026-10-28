# Implementation plan: a git-free "no new package under internal/" check for fix-7 (lm-steer-crswd-018-fix-7-fix-6)

Filed by `loop/fix-features.py` from a `verify-run.sh` FAIL (run `1022-tbfh`, PR #229,
parent task `lm-steer-crswd-018-fix-7`):

> Assertion: No new package under internal/ (git ls-tree diff)
> Evidence: The tree is not a git repo (`fatal: not a git repository`), so this could not be checked.

## Why it failed

`verify-run.sh` in ai-lawnmower copies the repo into a clean room with
`tar --exclude=.git` on purpose, so the verifier cannot read history. Line 30 of
fix-7's VALIDATION_CONTRACT.md is a `git ls-tree` diff of `main` against `HEAD`. With
no `.git` it cannot run, and an assertion the verifier cannot run is a FAIL. No Go
code is wrong. The contract line is.

Milestone `lm-steer-crswd-018-fix-16` fixed the identical bullet in the parent
milestone's contract. This milestone applies the same replacement to fix-7's contract.

## Facts measured at planning time (10/28/26, main `97a0f58`)

- `ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` is 31 lines and ends in a newline.
- Line 30 of that file is the line to replace. It begins `- No new package under \`internal/\`:`
  and is the only line in the file containing `git`.
- `grep -c '^- '` on that file prints `15`. `grep -c 'C-u'` on it prints `3` (lines 9, 14, 31).
  Replacing one bullet with one bullet keeps both counts.
- `internal/` holds 17 top-level directories at `97a0f58`, the same 17 fix-16 measured at
  `8a30044`: `access admit audit auth buildinfo claudeauth codexauth config harness httpapi
  loginrelay quota release session sessionpod tmuxctl updater`.
- D1's command was run in a `.git`-less copy of `internal/`: exit 0 and no output on the tree as
  it stands; exit 1 printing `< internal/zzzprobe` after `mkdir internal/zzzprobe`.
- `ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` line 37 carries the same
  `git ls-tree` bullet. It is out of scope here. A separate fix task owns it if the verifier
  flags it.
- Fix-7's T2 is `- [!]` (blocked). This milestone does not touch fix-7's plan, progress or prompt.
- Earlier loops (fix-16, fix-17 notebooks) found in the loop container: `bash -c '...'`, `$(...)`,
  pipes and `C=<path>; ... $C` are refused by the Bash guard; `.ralph-tmp/` is gitignored and is the
  scratch location that works; `safe-commit.sh` exists but `"$PWD"` is refused and the literal path
  needs approval, so PROMPT.md's fallback commit is what works.

## Decisions already made (do not reopen them)

- **D1 The replacement line.** Replace line 30 of `ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md`
  with exactly this one line, nothing else:

```text
- No new package under `internal/` (the seventeen on `main` at `97a0f58`; checked from the files alone, because the verifier's copy has no `.git`): `bash -c 'diff <(find internal -mindepth 1 -maxdepth 1 -type d | LC_ALL=C sort) <(printf "internal/%s\n" access admit audit auth buildinfo claudeauth codexauth config harness httpapi loginrelay quota release session sessionpod tmuxctl updater)'` exits 0 and prints nothing.
```

- **D2 Scope.** Top-level directories only, the same depth `git ls-tree -d internal/` checked.
  Do not widen it to nested packages.
- **D3 No product code.** No Go file, no test, no `docs/` file, no `specs/` file, no `AGENTS.md` change.

## Tasks

- [ ] T1 Replace line 30 of `ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` with D1's line verbatim; change no other line. Verify: `! grep -q "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` exits 0, and `grep -c '^- '` on that file prints `15`.
- [ ] F1 Run every assertion in this milestone's VALIDATION_CONTRACT.md as PROMPT.md's "How F1 runs a git-free assertion" says, and record each command, output and exit code in PROGRESS.md. Verify: `go build ./...` exits 0 and `go test ./...` exits 0.

## Files touched

The only path this milestone may change, besides its own notebook directory:

```allowlist
ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md
```

Not in scope, ever, in this milestone: any `.go` file, `internal/`, `cmd/`, `docs/`, `specs/`,
`k8s/`, `AGENTS.md`, `go.mod`, `go.sum`, every other file under `ralph/plans/lm-steer-crswd-018-fix-7/`,
and `ralph/plans/lm-steer-crswd-018-fix-9/`.

## Pre-commit commands

1. `git diff --name-only` prints only paths in "Files touched" or this notebook directory.
