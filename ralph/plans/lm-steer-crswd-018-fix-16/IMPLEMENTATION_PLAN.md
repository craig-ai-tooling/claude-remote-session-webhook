# Implementation plan: a git-free "no new package under internal/" check (lm-steer-crswd-018-fix-16)

Filed by `loop/fix-features.py` from a `verify-run.sh` FAIL (run `1014-la1i`, parent
task `lm-steer-crswd-018`):

> Assertion: no new package under internal/ (git ls-tree diff)
> Evidence: can't verify: the tree is not a git repo ("fatal: not a git repository")

## Why it failed

`verify-run.sh` in ai-lawnmower copies the repo into a clean room with
`tar --exclude=.git` on purpose, so the verifier cannot read history. The parent
contract's assertion is a `git ls-tree` diff of `main` against `HEAD`. With no
`.git` it cannot run, and an assertion the verifier cannot run is a FAIL. No Go code
is wrong. The contract line is.

## Facts measured at planning time (10/26/26, main `8a30044`)

- The line to replace is line 44 of `ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md`,
  the last bullet under "## The contract and the guardrails". It begins
  `- No new package under \`internal/\`:` and contains `git ls-tree`.
- That file has 24 lines starting with `- ` (`grep -c '^- '` prints `24`). Replacing one
  bullet with one bullet keeps 24.
- `git ls-tree -d --name-only` for `internal/` is identical at `9a938b4` (the parent's
  baseline) and `8a30044` (this branch's base): 17 directories, `access admit audit auth
  buildinfo claudeauth codexauth config harness httpapi loginrelay quota release session
  sessionpod tmuxctl updater`.
- The replacement command below was run in a `.git`-less copy of `internal/`: exit 0 and
  no output on the tree as it stands; exit 1 printing `< internal/zzzprobe` after
  `mkdir internal/zzzprobe`.
- `find ... | LC_ALL=C sort` and the `printf` list produce the same order. The list is
  already in C-locale order.
- Line 48 of the same file (the "After merge" bullet) also calls `git`. It sits under
  "checked by the merging session, not by the loop" and is out of scope here.

## Decisions already made

- **D1 The replacement line.** Replace line 44 with exactly this one line, nothing else:

```text
- No new package under `internal/` (the seventeen on `main` at `8a30044`; checked from the files alone, because the verifier's copy has no `.git`): `bash -c 'diff <(find internal -mindepth 1 -maxdepth 1 -type d | LC_ALL=C sort) <(printf "internal/%s\n" access admit audit auth buildinfo claudeauth codexauth config harness httpapi loginrelay quota release session sessionpod tmuxctl updater)'` exits 0 and prints nothing.
```

- **D2 Scope.** Top-level directories only, the same depth `git ls-tree -d internal/`
  checked. Do not widen it to nested packages.
- **D3 No product code.** No Go file, no test, no `docs/` file, no `AGENTS.md` change.

## Tasks

- [x] T1 In `ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md`, replace the `git ls-tree` bullet with D1's line verbatim; change no other line. Verify: `bash -c 'C=ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md; ! grep -q "git ls-tree" $C && grep -c "^- " $C | grep -qx 24 && grep -qF "tmuxctl updater)" $C'` exits 0.
- [x] F1 Run every command in this milestone's VALIDATION_CONTRACT.md, from a `.git`-less copy where it names git-free checks, and record each result in PROGRESS.md. Verify: `go build ./... && go test ./...` exits 0.

## Files touched

The only path this milestone may change, besides its own notebook directory:

```allowlist
ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md
```

Not in scope, ever, in this milestone: any `.go` file, `internal/`, `cmd/`, `docs/`,
`specs/`, `k8s/`, `AGENTS.md`, `go.mod`, `go.sum`, and every other file under
`ralph/plans/lm-steer-crswd-018/`.

## Pre-commit commands

1. `git diff --name-only` prints only paths in "Files touched" or this notebook directory.
