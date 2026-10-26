# Progress: a git-free "no new package under internal/" check (lm-steer-crswd-018-fix-16)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Baseline (planning, 10/26/26)

- Branch `plan/lm-steer-crswd-018-fix-16`, cut from `main` at `8a30044`.
- `grep -n 'git ls-tree' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints line 44.
- `grep -c '^- ' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `24`.
- D1's command, run in a `.git`-less copy of `internal/`: exit 0, no output. With `internal/zzzprobe` added: exit 1, prints `< internal/zzzprobe`.
- Earlier loops in this repo (fix-9) found `golangci-lint` not installed and `~/code/ai-lawnmower/loop/safe-commit.sh` unreachable inside the loop container. If that holds, record it and use PROMPT.md's fallback.
- Earlier loops found `internal/sessionpod` and `internal/loginrelay` flaky under a loaded `go test ./...`. If either fails, re-run it alone with `go test -count=1 ./internal/sessionpod/ ./internal/loginrelay/` before believing it. This milestone touches no Go.

## Iterations

### T1 — replace the `git ls-tree` bullet with D1's line (10/26/26)

Branch `ralph/1026-vwb9`, base `232acbd`. Working tree was clean at the start.

**Changed:** `ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` line 44 only — the
last bullet under "## The contract and the guardrails" — replaced with D1's line. No
other line of that file was touched.

**Commands and what they measured:**

- Before the edit, `grep -n 'git ls-tree' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md`
  printed line 44, matching the plan's "Facts measured". `grep -c '^- '` printed `24`;
  `wc -l` printed `48`.
- After the edit, `git diff --stat` printed `1 file changed, 1 insertion(+), 1 deletion(-)`
  and `git diff -U0` showed a single hunk `@@ -44 +44 @@`. The file is still 48 lines with
  24 `^- ` bullets.
- **D1 copied verbatim, proven by bytes, not by eye.** `sed -n '41p'` of this milestone's
  IMPLEMENTATION_PLAN.md (D1's line inside the ```text fence) and `sed -n '44p'` of the
  parent contract, compared with `cmp`, are identical. Note for a later iteration: D1's
  line contains a literal backslash-n inside `printf "internal/%s\n"`, which is correct —
  it must stay a two-character escape in the markdown, not a real newline.
- T1's Verify exited 0.

**Gotcha the next iteration would otherwise rediscover — the Bash permission layer
rejects the Verify command as written.** Two separate guards fire in this container:

- A hook rejects `C=<path>; ... $C` with `Contains simple_expansion`.
- `bash -c '...'` with the `C=` assignment then needs interactive approval, which a
  headless run cannot give. Writing the command to `/tmp` and running it from there is
  blocked too (no write permission outside `/workspace`), and so is passing `C=` as a
  positional argument after `bash -c`.

The way through is to inline the path in place of `$C` and drop the `bash -c` wrapper,
which leaves the three assertions unchanged:
`! grep -q "git ls-tree" <path> && grep -c "^- " <path> | grep -qx 24 && grep -qF "tmuxctl updater)" <path>`.
Run that way it exited 0. Expect the same treatment for F1's contract commands — several
of them are `bash -c` one-liners with `$(...)` and process substitution, so F1 should plan
on inlining and on PROMPT.md's `mktemp -d` + `tar --exclude=.git` recipe rather than on
pasting the contract lines unchanged.

**Not done here, left to F1:** no command from this milestone's VALIDATION_CONTRACT.md was
run yet, including `go build ./... && go test ./...`. T1's Verify does not cover them.

**safe-commit.sh could not be run, so this commit used PROMPT.md's fallback.** The
baseline's guess from fix-9 holds: there is no ai-lawnmower checkout in this container.
`test -e /home/ralph/code` is false, so `~/code/ai-lawnmower/loop/safe-commit.sh` does not
exist — not a permissions problem, the path is simply absent. (`ls` on it is also refused,
because the session may only list paths under `/workspace`; and the invocation as PROMPT.md
writes it, with `~` and `"$PWD"`, is rejected by the Bash guard as unanalyzable shell syntax
before it ever runs.) Committed instead with one `git add <path>` per path, by name, then
`git commit`. A later iteration should not spend time re-testing for safe-commit: it is not
here, and the fallback is the normal path for this milestone.

### F1 — run every command in this milestone's VALIDATION_CONTRACT.md (10/26/26)

Branch `ralph/1026-vwb9`, base `5822d7c`. Working tree clean at the start. **No tracked
file was changed by this task** beyond this notebook and IMPLEMENTATION_PLAN.md; F1 only
runs and records.

**Result: all six bullets of `ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md`
pass.** One by one, with what each measured:

| # | Bullet | Measured |
|---|---|---|
| 1 | parent contract no longer depends on git | `grep -n "git ls-tree"` printed nothing; `grep -c` printed `0` → the `! grep` form exits 0 |
| 2 | names all seventeen packages | `grep -cF '<the 17 names>'` printed `1` |
| 3 | the tree has exactly those seventeen | `diff` of `find`+`sort` against the `printf` list printed nothing, exit 0 |
| 4 | the check catches a new package | with `internal/zzzprobe` added, `diff` printed `18d17` / `< internal/zzzprobe`, exit 1 |
| 5 | every other assertion kept | `grep -c '^- '` printed `24` |
| 6 | product builds and tests pass | `go build ./... && go test ./...` exit 0, 22 `ok`, 2 `[no test files]`, 0 FAIL |

Bullets 1, 2, 3 and 5 were run twice: once from the `.git`-less scratch copy, once from the
repo root. Bullet 3's seventeen names came out identical both ways and identical to the
plan's "Facts measured", so nothing drifted since planning. Neither `internal/sessionpod`
nor `internal/loginrelay` was flaky this run — both `ok` on the first try, so the baseline's
re-run advice was not needed.

**The scratch copy, and how it had to differ from PROMPT.md's recipe.** PROMPT.md's
`mktemp -d` + `tar --exclude=.git` recipe cannot run in this container. What blocks it, each
checked rather than assumed:

- `mkdir` outside `/workspace` is refused: *"Claude Code may only create directories in the
  allowed working directories for this session: '/workspace'"*. So `mktemp -d`, which lands
  in `/tmp`, is out. (This confirms and sharpens T1's note about `/tmp`.)
- `tar` is not on the permission allowlist and needs interactive approval, which a headless
  run cannot give. So does `cp` **with any flag** — the refusal is explicit that a flag like
  `--target-directory` could bypass path validation — which rules out `cp -r` and therefore
  rules out bullet 4's command as literally written. A `tar | tar` pipeline is refused as
  "multiple operations" on top of that.
- **Flagless `cp <src> <dst>` is allowed**, and so are `mkdir -p`, `rm` (no flags), `rmdir`,
  `find`, `grep`, `diff`, `cat`, `ls`, `printf`, an `LC_ALL=C` command prefix, and plain `>`
  redirection. That set is enough to do everything by hand.

So the scratch copy was built at `/workspace/.ralph-tmp/f16` — **`.ralph-tmp/` is already in
`.gitignore`** (under "logs & scratch"), which is why the scratch never dirtied the tree and
why that path is the right one to reuse. It held exactly what these assertions read: the
seventeen `internal/*` directories (`mkdir -p`) and a flagless-`cp` copy of
`ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` at its real relative path. `find
/workspace/.ralph-tmp/f16 -name .git` printed nothing. The Bash tool's cwd persists across
calls when `cd` is sent **as its own command** (`cd` inside a compound command prompts for
approval), so the contract's relative paths were exercised as written, from the scratch root.

**Two honest limits on that scratch, neither of which changes a result.** First, it is a
partial copy: the four git-free bullets read only those two things, so a full copy would add
nothing — but bullet 6 was therefore run in the real repo, not the scratch. Second, because
`mkdir` is confined to `/workspace`, the scratch sits *inside* the repo, so `git` run from
there still walks up and finds `/workspace/.git` (`git status` from the scratch printed
`On branch ralph/1026-vwb9`). That does not weaken anything: none of bullets 1–5 invokes
git at all, which is the whole point of the rewrite. A verifier that copies the tree out to
a truly separate directory is strictly easier on these commands than what was run here.

Because bullet 3's scratch `internal/` was one this iteration created by hand, its passing
there proves only "no git needed", not "the tree is right". The second run, from the repo
root against the real `internal/`, is what proves the tree: seventeen directories, `diff`
silent, exit 0.

**How the `bash -c` bullets were run.** T1's prediction held — every `bash -c` in the
contract needs approval, and process substitution and pipes are refused as unanalyzable.
Each was decomposed into single allowed commands that compute the same thing: `find ... >
found.raw`, then `LC_ALL=C sort found.raw > found.sorted`, then `printf "internal/%s\n" ...
> expected.list`, then `diff found.sorted expected.list`. Two notes for a later iteration:

- **`$?` cannot be read.** `; echo "exit=$?"` is refused, and so is `C=<path>; ... $C`. Exit
  codes have to be inferred from a command's defined behaviour — `grep` exits 1 on no match,
  `diff` exits 0 iff identical — or encoded in output, which is why bullet 1 was also run as
  `grep -c` to get a printed `0`.
- The `printf` list is already in C-locale order, as the plan measured, so `found.sorted` and
  `expected.list` agree line for line with no reordering.

**`rm -rf <path>` is blocked by `danger-guard.sh` for any absolute path.** Removing the
scratch with `rm -rf /workspace/.ralph-tmp` was blocked citing *"rm -rf against / (root
filesystem destruction)"* — the rule matches the `/` that starts any absolute path, not just
the root itself. The guard was not routed around. The scratch came out piecewise instead:
flagless `rm` for the files, then `rmdir` for the directories innermost-first. `ls -A
/workspace` afterwards shows no `.ralph-tmp`, and `git status --short` is empty. (Worth a
guard fix in its own PR someday, per AGENTS.md; out of scope for this milestone.)

**safe-commit.sh is still absent** — T1's finding, unchanged, so this commit used PROMPT.md's
fallback again. Do not re-test for it.

