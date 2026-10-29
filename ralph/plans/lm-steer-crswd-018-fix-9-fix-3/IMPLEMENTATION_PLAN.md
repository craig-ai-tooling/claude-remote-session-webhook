# Implementation plan: a git-free "no new package under internal/" check for fix-9 (lm-steer-crswd-018-fix-9-fix-3)

Filed by `loop/fix-features.py` from a `verify-run.sh` FAIL (run `1022-6xpl`, PR #234,
parent task `lm-steer-crswd-018-fix-9`):

> Assertion: No new package under `internal/` (git ls-tree diff vs main)
> Evidence: The tree is not a git repo, so both `git ls-tree` calls fail with "fatal: not a git
> repository". The diff exits 0 only because it compares two empty outputs.

## Why it failed

`verify-run.sh` in ai-lawnmower copies the repo into a clean room with
`tar --exclude=.git` on purpose, so the verifier cannot read history. Line 37 of
fix-9's VALIDATION_CONTRACT.md is a `git ls-tree` diff of `main` against `HEAD`. With
no `.git` both sides are empty, so the check proves nothing. No Go code is wrong. The
contract line is.

Milestone `lm-steer-crswd-018-fix-7-fix-6` fixed the identical bullet in fix-7's
contract (merged as `5ca1507`). This milestone applies the same replacement to fix-9's contract.

## Facts measured at planning time (10/29/26, main `5ca1507`)

- `ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` is 37 lines and ends in a newline.
- Line 37 (the last line) is the line to replace. It begins `- No new package under \`internal/\`:`
  and is the only line in the file containing `git`.
- `grep -c '^- '` on that file prints `13`. `grep -c 'leading'` on it prints `3`.
  Replacing one bullet with one bullet keeps both counts.
- `internal/` holds 17 top-level directories at `5ca1507`, the same 17 fix-7-fix-6 measured at
  `97a0f58`: `access admit audit auth buildinfo claudeauth codexauth config harness httpapi
  loginrelay quota release session sessionpod tmuxctl updater`.
- D1's command, run with `bash -c` on the tree: exit 0, no output. In a scratch copy of `internal/`
  with `zzzprobe` added: exit 1, output `18d17` / `< internal/zzzprobe`.
- `ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` line 30 already carries the
  git-free form of this check. D1 below is that line with the commit pinned to `5ca1507`.
- All four of fix-9's tasks are `- [x]`. This milestone does not touch fix-9's plan, progress,
  prompt or STORY_CLASSES.json.
- Earlier loops (fix-16, fix-17, fix-7-fix-6 notebooks) found in the loop container: `bash -c '...'`,
  `$(...)`, pipes and `C=<path>; ... $C` are refused by the Bash guard; `.ralph-tmp/` is gitignored
  (`.gitignore` line 36) and is the scratch location that works; `safe-commit.sh` exists but
  `"$PWD"` is refused and the literal path needs approval, so PROMPT.md's fallback commit is what works.

## Decisions already made (do not reopen them)

- **D1 The replacement line.** Replace line 37 of `ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md`
  with exactly this one line, nothing else, and keep the trailing newline:

```text
- No new package under `internal/` (the seventeen on `main` at `5ca1507`; checked from the files alone, because the verifier's copy has no `.git`): `bash -c 'diff <(find internal -mindepth 1 -maxdepth 1 -type d | LC_ALL=C sort) <(printf "internal/%s\n" access admit audit auth buildinfo claudeauth codexauth config harness httpapi loginrelay quota release session sessionpod tmuxctl updater)'` exits 0 and prints nothing.
```

- **D2 Scope.** Top-level directories only, the same depth `git ls-tree -d internal/` checked.
  Do not widen it to nested packages.
- **D3 No product code.** No Go file, no test, no `docs/` file, no `specs/` file, no `AGENTS.md` change.

## Tasks

- [x] T1 Replace line 37 of `ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` with D1's line verbatim; change no other line. Verify: `! grep -q "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` exits 0, and `grep -c '^- '` on that file prints `13`.
- [ ] F1 Run every assertion in this milestone's VALIDATION_CONTRACT.md as PROMPT.md's "How F1 runs a git-free assertion" says, and record each command, output and exit code in PROGRESS.md. Verify: `go build ./...` exits 0 and `go test ./...` exits 0.

## Files touched

The only path this milestone may change, besides its own notebook directory:

```allowlist
ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md
```

Not in scope, ever, in this milestone: any `.go` file, `internal/`, `cmd/`, `docs/`, `specs/`,
`k8s/`, `AGENTS.md`, `go.mod`, `go.sum`, every other file under `ralph/plans/lm-steer-crswd-018-fix-9/`,
and everything under `ralph/plans/lm-steer-crswd-018-fix-7/`.

## Pre-commit commands

1. `git diff --name-only` prints only paths in "Files touched" or this notebook directory.
