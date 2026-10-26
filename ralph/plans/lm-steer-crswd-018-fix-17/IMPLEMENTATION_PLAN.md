# Implementation plan: move the post-merge daemon check out of the black-box contract (lm-steer-crswd-018-fix-17)

Filed by `loop/fix-features.py` from a `verify-run.sh` FAIL (run `1014-la1i`, parent
task `lm-steer-crswd-018`):

> Assertion: running daemon at merge commit or later
> Evidence: can't verify: there is no git history or merge commit to check against. This one belongs to the merging session

## Why it failed

`verify-run.sh` in ai-lawnmower copies the repo into a clean room with
`tar --exclude=.git` and counts every `- ` or `* ` bullet of 16+ characters in
VALIDATION_CONTRACT.md as an assertion. A bullet the verifier cannot run is a FAIL.
The parent contract's last bullet (line 48, under "## After merge (checked by the
merging session, not by the loop)") reads the live daemon's `vcs.revision` through
`systemctl` and `/proc`, then calls `git merge-base`. The clean room has no `.git`,
no merge commit and no daemon. The heading saying "not by the loop" does not help:
the parser counts the bullet anyway. No Go code is wrong. The contract line is in
the wrong file.

## Facts measured at planning time (10/26/26, main `c133170`)

- `ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` is 48 lines and ends in a newline.
  Line 44 is the last bullet under "## The contract and the guardrails". Line 45 is blank.
  Line 46 is `## After merge (checked by the merging session, not by the loop)`. Line 47 is
  blank. Line 48 is the failing bullet, beginning `- The running daemon is at the merge commit or later:`.
- `grep -c '^- '` on that file prints `24`. `head -44` of it gives `23`.
- On `head -44` of that file, `grep -cE 'merge-base|MainPID|vcs.revision'` prints `0`, and
  `grep -cE '^- .*git (merge-base|ls-tree|log|diff|rev-parse|show)'` prints `0`.
- `ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` is 133 lines. Its last section is
  "## After the loop (a session does these, not Ralph)". Line 133 is exactly
  `3. Run the "After merge" check in VALIDATION_CONTRACT.md.` It is the only line in the
  parent's notebook that points at the "After merge" section.
- `ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md` line 16 asserts the parent
  contract has `24` bullets. Removing the bullet makes it `23`, so that line must change too.
- `verify-run.sh` counts bullets with `^\s*[-*]\s+(.{16,})$`. Numbered list items (`1. `) and
  prose are not counted.
- fix-16's PROGRESS.md records, for the loop container: `~/code/ai-lawnmower/loop/safe-commit.sh`
  is absent; `bash -c '...'`, `$(...)`, pipes and `C=<path>; ... $C` are refused by the Bash
  guard; `rm -rf` on an absolute path is blocked by `danger-guard.sh`. Inline paths and run
  each piece of a check as its own command.

## Decisions already made

- **D1 Remove, do not rewrite.** Delete lines 45 through 48 of the parent contract (the blank
  line, the "After merge" heading, the blank line, the bullet). The file then ends at line 44
  with a newline. `sed -i '45,48d' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` does it.
  Do not replace the bullet with another bullet: there is no git-free form of "the running
  daemon is at the merge commit".
- **D2 Keep the check, in the parent's "After the loop".** Replace line 133 of the parent
  IMPLEMENTATION_PLAN.md with exactly this one line, nothing else:

```text
3. Confirm the running daemon is at the merge commit or later: `bash -c 'go version -m /proc/$(systemctl --user show -p MainPID --value crswd)/exe | grep vcs.revision'` prints a revision R for which `git merge-base --is-ancestor <merge commit> R` exits 0. It needs `.git` and the live daemon, so it lives here and not in VALIDATION_CONTRACT.md, whose verifier has neither.
```

- **D3 fix-16's count.** In `ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md`,
  replace line 16 with exactly this one line, nothing else:

```text
- The parent milestone's contract kept every other assertion (24 when fix-16 ran; fix-17 then moved the post-merge daemon check out, leaving 23): `grep -c '^- ' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `23`.
```

- **D4 No product code.** No Go file, no test, no `docs/` file, no `specs/` file, no `AGENTS.md` change.

## Tasks

- [x] T1 Apply D1 to `ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md`. Verify: `! grep -qE "merge-base|MainPID|^## After merge" ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` exits 0, `grep -c '^- ' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `23`, and `wc -l < ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `44`.
- [x] T2 Apply D2: replace line 133 of `ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` with D2's line verbatim. Verify: `grep -c 'merge-base --is-ancestor' ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` prints `1` and `grep -c '"After merge" check' ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` prints `0`.
- [x] T3 Apply D3: replace line 16 of `ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md` with D3's line verbatim. Verify: `grep -c "prints \`23\`" ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md` prints `1` and `grep -c "prints \`24\`" ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md` prints `0`.
- [x] F1 Run every command in this milestone's VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go build ./... && go test ./...` exits 0.

## Files touched

The only paths this milestone may change, besides its own notebook directory:

```allowlist
ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md
ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md
ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md
```

Not in scope, ever, in this milestone: any `.go` file, `internal/`, `cmd/`, `docs/`,
`specs/`, `k8s/`, `AGENTS.md`, `go.mod`, `go.sum`, the parent's PROGRESS.md, PROMPT.md and
STORY_CLASSES.json, and every other file under `ralph/plans/lm-steer-crswd-018-fix-16/`.

## Pre-commit commands

1. `git diff --name-only` prints only paths in "Files touched" or this notebook directory.
