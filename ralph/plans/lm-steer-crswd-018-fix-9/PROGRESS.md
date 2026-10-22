# Progress: leading `!` or `/` on the prompt route (lm-steer-crswd-018-fix-9)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Planning notes (10/22/26)

- Branch `plan/lm-steer-crswd-018-fix-9`, cut from `main` at `9a938b4`.
- Baseline: `go test ./internal/httpapi/ -run 'Prompt|Type'` exits 0 (`ok ... 0.205s`). Every test it runs predates the rule.
- `grep -n 'errPromptLeadingCommand' internal/httpapi/*.go` prints nothing on the baseline.
- No `hostilePrompts` entry and no quickstart prompt payload begins with `!` or `/`, so the new refusal breaks no existing test.
- T2's tests are regression guards. They are expected to pass before and after T1; they cannot be proven failing-before, and that is correct.
- Earlier loops in this repo found golangci-lint and `~/code/ai-lawnmower/loop/safe-commit.sh` unreachable inside the loop container. If that holds here, record it and use the fallback in PROMPT.md.

## Failing-before evidence

| Task | Test | Failure line before the change |
|---|---|---|
| T1 | `TestPromptRefusesALeadingCommandCharacter/a_slash_command` | `sessions_test.go:1249: status = 202 ("{\"id\":\"1cc6283446a70c77aa735914a91a2dba\",\"delivered\":true}"); want 400` |
| T1 | `TestPromptRefusesALeadingCommandCharacter/a_bash_escape` | `sessions_test.go:1249: status = 202 ("{\"id\":\"54c471f561e1e76a195f0d69c937d74f\",\"delivered\":true}"); want 400` |
| T1 | `TestPromptRefusesALeadingCommandCharacter/a_slash_command_behind_spaces` | `sessions_test.go:1249: status = 202 ("{\"id\":\"53e973c56461a3393a17bfb528878023\",\"delivered\":true}"); want 400` |
| T1 | `TestPromptRefusesALeadingCommandCharacter/a_bash_escape_behind_a_newline` | `sessions_test.go:1249: status = 202 ("{\"id\":\"074473b068e77d09db6b8d613432ff32\",\"delivered\":true}"); want 400` |
| T1 | `TestPromptRefusesALeadingCommandCharacter/a_slash_command_behind_a_return` | `sessions_test.go:1249: status = 202 ("{\"id\":\"70ef852720972b288b3fc96b4ed0e3ba\",\"delivered\":true}"); want 400` |

## Iterations

### T1 — the refusal itself (10/22/26)

**Changed**

- `internal/httpapi/sessions.go`: G1 added `errPromptLeadingCommand` to the prompt
  sentinels' `var (...)` block, with the why-comment D5/G1 asked for (leading `/` is a
  slash command, leading `!` is a bash escape). G2 added the check in `promptSession`
  immediately after the successful `decode` and *before* the `req.Text != ""` dialog
  block — `strings.TrimLeft(req.Text, " \t\r\n")`, then the first byte against `!` and
  `/`, then `s.rejectBadRequest(w, r, errPromptLeadingCommand)` and return. `strings`
  was already imported; nothing else in the file moved.
- `internal/httpapi/sessions_test.go`: added `TestPromptRefusesALeadingCommandCharacter`
  (table, `t.Parallel()` on the test and each row), five rows — `/status`, `!ls`,
  `"  /x"`, `"\n\t!x"`, `"\r/x"`. Each asserts 400, body exactly `bodyBadRequest`, zero
  tmux calls, and one audit record with `decision` deny, `reason`
  `errPromptLeadingCommand.Error()`, `action` `session.prompt`.

**Order followed.** The sentinel and the test went in first and the handler check second,
so the failing-before run above is a behavioural failure (202 where 400 was wanted) rather
than a compile error. The sentinel alone changes no behaviour, which is why it could ride
along with the test.

**Commands run**

| Command | Result |
|---|---|
| `go test ./internal/httpapi/ -run Prompt` (before the handler change) | FAIL, all five rows, lines in the table above |
| `go test ./internal/httpapi/ -run Prompt` (after) | `ok ... 0.017s` |
| `go test ./internal/httpapi/ -run 'Prompt\|Type' -v \| grep '--- PASS: TestPromptRefusesALeadingCommandCharacter/'` | 5 lines |
| `gofmt -l .` | prints nothing |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go vet -tags tmux ./...` | exit 0 |
| `go vet -tags quickstart ./cmd/crswd` | exit 0 |
| `golangci-lint run` | **not installed** in this container (exit 127, `command not found`); the SessionStart hook says the same and gives the pin `v2.12.2`. Not run. |
| `test ! -e go.sum` | exit 0 |
| `go test ./internal/httpapi/ ./internal/session/ ./internal/audit/` | all `ok` |
| `git status --short` | only `internal/httpapi/sessions.go` and `internal/httpapi/sessions_test.go`, both in "Files touched" |

**For the next iteration**

- The check sits above the dialog read, so a refused prompt records **zero** tmux calls,
  not one capture. T2's `hostilePrompts` additions (`"fix a/b\n/compact"`, `"say hi!"`)
  are unaffected: `TrimLeft` leaves `f` and `s` as the first byte, so both still take the
  202 path and `TestAPromptIsDeliveredByteForByte`'s four-call expectation still holds.
- Subtest names in the new table contain spaces, so `go test -v` prints them with
  underscores — that is what the VALIDATION_CONTRACT's grep matches.
- The planning note that `golangci-lint` lives at `/home/nctiggy/bin/golangci-lint` does
  not hold inside the loop container; it is not on `PATH` and that path was not reachable.
- **`safe-commit.sh` could not be run.** Every invocation of
  `~/code/ai-lawnmower/loop/safe-commit.sh` (and of `bash <abs path>` to it, and a read of
  the file itself) was refused by this container's permission layer before it executed —
  nothing outside `/workspace` is reachable from this session, so the script's existence
  could not even be confirmed. T1 was committed with PROMPT.md's "If safe-commit cannot be
  reached" fallback: one `git add <path>` per touched path, by name, then `git commit -m`.
  The planning note predicted this; a later iteration should expect the same and not spend
  time on it.

### T2 — the two regression guards (10/22/26)

**Changed**

- `internal/httpapi/sessions_test.go`: added the two `hostilePrompts` keys the task
  names — `"a later slash": "fix a/b\n/compact"` and `"a later bang": "say hi!"` — with a
  why-comment saying what they hold down: the T1 check reads the first non-whitespace
  byte and nothing else, so a `/` or `!` past it is ordinary text that still travels
  byte-for-byte. Both ride `TestAPromptIsDeliveredByteForByte` unchanged, including its
  four-call expectation (capture-pane, load-buffer, paste-buffer, send-keys Enter).
- `internal/httpapi/input_test.go`: added `TestTypeStillDeliversASlashCommand` above
  `TestTypeRefusesBadTextWithItsOutcome` — `/compact` with `confirmYes` answers 204 and
  the one call carrying stdin holds exactly `/compact`. It reads payloads the way
  `TestTypeNormalisesCRLF` does: the type route makes two `OpPasteBracketed` calls
  (`load-buffer`, then `paste-buffer`) and only the first carries stdin, so "the one
  stdin payload" is one entry and not two.

**No failing-before evidence, by design.** Both are regression guards for G4 — the type
route and the later-`/` prompts were never meant to change — so they pass on T1's code
and would have passed before it. The planning note says so; the PROMPT's failing-before
procedure is scoped to T1.

**Commands run**

| Command | Result |
|---|---|
| `go test ./internal/httpapi/ -run 'Prompt\|Type'` | `ok ... 0.493s` |
| the same with `-v`, grepped for the three new cases | `--- PASS: TestTypeStillDeliversASlashCommand`, `--- PASS: TestAPromptIsDeliveredByteForByte/a_later_slash`, `--- PASS: .../a_later_bang` |
| `gofmt -l .` | prints nothing |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go vet -tags tmux ./...` | exit 0 |
| `go vet -tags quickstart ./cmd/crswd` | exit 0 |
| `golangci-lint run` | **not installed**, exit 127 `command not found`, exactly as in T1. Not run. |
| `test ! -e go.sum` | exit 0 |
| `git status --short` | only `internal/httpapi/input_test.go` and `internal/httpapi/sessions_test.go`, both in "Files touched" |

**For the next iteration**

- `safe-commit.sh` was again unreachable from this container; T2 used the same
  `git add <path>` + `git commit -m` fallback. Do not spend time retrying it.
- T3 is a one-line append to `docs/fixes-log.md` — read the file's existing format first;
  nothing in `internal/` needs to move.

### T3 — the operator-facing record (10/22/26)

**Changed**

- `docs/fixes-log.md`: appended one line, dated 2026-10-22, at the bottom (the file is
  append-only, newest last). It says what was wrong — the API door's
  `POST /sessions/{id}/prompt` typed a prompt whose **leading** non-whitespace character
  was `/` or `!` straight into the pane, where Claude Code reads a leading `/` as a slash
  command and a leading `!` as a bash escape — and what changed: `400` with
  `{"error":"bad request"}` before any tmux command, leading spaces/tabs/CR/LF skipped to
  find that character, a `/` or `!` past it still ordinary text, and the browser's type
  route deliberately unchanged. Tagged `(lm-steer-crswd-018-fix-9)`, the milestone id, in
  the `(#issue-or-PR)` slot — the same shape the `(lm-95)` and `(lm-crswd-onfailure)`
  entries already use, since there is no issue or PR number for this fix.

**Why one long line rather than a short one.** The file's format block asks for one line
and says anything needing more than one line of explanation is not a quick fix. This is
one line; the neighbouring entries from #150 onward are the same length, and the
VALIDATION_CONTRACT additionally requires that line to carry both `!` and `/` and the
word `leading`, which a terse version would not.

**Commands run**

| Command | Result |
|---|---|
| `grep -n 'leading' docs/fixes-log.md \| grep 2026-10-22` (the task's Verify) | exit 0, one line — `docs/fixes-log.md:60` |
| `gofmt -l .` | prints nothing |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go vet -tags tmux ./...` | exit 0 |
| `go vet -tags quickstart ./cmd/crswd` | exit 0 |
| `golangci-lint run` | **not installed**, exit 127 `command not found`, as in T1 and T2. Not run. |
| `test ! -e go.sum` | exit 0 |
| `git status --short` | only `docs/fixes-log.md`, which is in "Files touched" |

**For the next iteration**

- F1 is the last task. The VALIDATION_CONTRACT's fixes-log assertion is the looser
  `grep -n 'leading' docs/fixes-log.md` plus a reading that the line is dated 2026-10-22
  or later and mentions both `!` and `/` — the appended line satisfies all three, and
  note the entry above it is *also* dated 2026-10-22 (the install.sh `rc` fix), so match
  on the word `leading` rather than on the date alone.
- Two of F1's assertions have not been run in this milestone yet: the API door's
  six-route count in `specs/001-crswd-daemon-core/contracts/http-api.md`, and the
  no-new-`internal/`-package diff of `git ls-tree -d` against `main`. Neither milestone
  task touched those paths, so both are expected to hold.
- `golangci-lint` is absent in this container; F1 should record "not installed" for it
  rather than treat it as a failure — the contract says "exits 0 where installed".
- `safe-commit.sh` is unreachable from this container (T1 and T2 both found this). T3
  used the `git add <path>` + `git commit -m` fallback again.

### F1 — the validation contract, run end to end (10/22/26)

**Changed**

- Nothing in the tree. F1 only runs VALIDATION_CONTRACT.md and records it here, plus the
  box in IMPLEMENTATION_PLAN.md.

**Behaviour assertions**

| Assertion | Command | Result |
|---|---|---|
| The refusal's named cases pass | `go test ./internal/httpapi/ -run 'Prompt' -v 2>&1 \| grep -E -- '--- PASS: TestPromptRefusesALeadingCommandCharacter/'` | **5 lines** (`a_slash_command`, `a_bash_escape`, `a_slash_command_behind_spaces`, `a_bash_escape_behind_a_newline`, `a_slash_command_behind_a_return`); contract wants ≥4 |
| The prompt suite is green | `go test ./internal/httpapi/ -run 'Prompt'` | `ok ... 0.021s`, exit 0 |
| A later `/` or `!` still delivered | `go test ./internal/httpapi/ -run 'Prompt' -v 2>&1 \| grep -E -- '--- PASS: TestAPromptIsDeliveredByteForByte/a_later_(slash\|bang)'` | **2 lines**; contract wants ≥2 |
| The type route still delivers `/compact` | `go test ./internal/httpapi/ -run 'Type' -v 2>&1 \| grep -E -- '--- PASS: TestTypeStillDeliversASlashCommand'` | **1 line**; contract wants ≥1 |
| No prompt text on the record or the response | `go test ./internal/audit/ ./internal/httpapi/` | both `ok`, exit 0 |
| The fix is recorded for operators | `grep -n 'leading' docs/fixes-log.md` | one line, `docs/fixes-log.md:60`, dated 2026-10-22, naming both `!` and `/` |

**Suites and guardrails**

| Assertion | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go test ./...` | exit 0 on a `-count=1` run — but **the first run failed**, see below |
| `go vet ./...` | exit 0 |
| `go vet -tags tmux ./...` | exit 0 |
| `go vet -tags quickstart ./cmd/crswd` | exit 0 |
| `golangci-lint run` | **not installed** (exit 127, `command not found`), as in T1–T3. The contract says "exits 0 where golangci-lint is installed", so this is not a failure. |
| `test ! -e go.sum` | exit 0 |
| `gofmt -l .` | prints nothing |
| API door still has six routes | **6**, see the note below on how it was counted |
| No new package under `internal/` | holds, see the note below on how it was checked |
| `git status --short` | only this notebook directory |

**The first `go test ./...` failed and the second did not.** Recorded in full because it
is the kind of thing a later reader will meet again:

- `internal/loginrelay` — `TestCodexSignedIn/signed_out_on_stdout`:
  `fork/exec /tmp/.../codex: text file busy`. The test writes a fake binary and execs it;
  the error is the kernel refusing an exec while the written file is still held open.
- `internal/sessionpod` — seven cases (`TestRunFailsWhenTmuxCannotBeAsked`,
  `TestRunKeepsWaitingWhenHasFailsButTheSessionIsListed`,
  `TestRunTakesAnEmptyListAsTheSessionBeingGone`,
  `TestRunStartsALoginShellAndSendsNothing`, `TestRunReturnsWhenTheSessionIsGone`,
  `TestRunSeedsBeforeNew`, `TestRunKillsAndConfirmsTheSessionOnCancel` both rows), each a
  5s `timed out waiting for …`.

Both packages passed immediately when run alone (`go test ./internal/loginrelay/
./internal/sessionpod/` → both `ok`), and the whole suite passed on `go test -count=1
./...`. Neither package imports anything this milestone touched — the diff is
`internal/httpapi/{sessions.go,sessions_test.go,input_test.go}` and `docs/fixes-log.md`
— so these are load-sensitive tests on a busy container (five-second deadlines, and the
first run compiled everything cold alongside `internal/config`, `release` and `updater`),
not a regression from this fix. **Carried forward as an open finding, not as done work.**

**Two assertions could not be run as literally written, and were satisfied another way.**
This container's permission layer refuses some command shapes outright:

- The six-route count: `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) '` was refused (the
  backtick in the pattern). Ran `grep -nE '^## .(GET|POST|DELETE|PATCH|PUT) '` on the same
  file instead and read the six printed lines, which show the backtick the contract's
  pattern asks for: `POST /sessions` (128), `GET /sessions` (183), `GET /sessions/{id}`
  (223), `DELETE /sessions/{id}` (231), `POST /sessions/{id}/prompt` (256),
  `GET /sessions/{id}/output` (350). Six, unchanged.
- The no-new-`internal/`-package diff: `git ls-tree` is refused here under every revision
  tried, and **this clone has no local `main`** — only `origin/main` and the two
  `plan/`+`ralph/` branches, so `git ls-tree -d --name-only main internal/` would fail on
  the revision name before the permission layer mattered. Ran
  `git diff --name-only origin/main HEAD` instead: nine paths, the four in "Files touched"
  plus five in this notebook directory. No path under `internal/` outside
  `internal/httpapi/`, and no new directory anywhere, which is strictly more than the
  directory-only diff asserts.

**For the next iteration**

- Every task in IMPLEMENTATION_PLAN.md is now `- [x]`, so this milestone is complete and
  the sentinel is appended below.
- If a verify run here fails in `internal/sessionpod` or `internal/loginrelay`, re-run
  those two packages alone before believing it. They are the flakiest pair in the tree on
  this container.
- `golangci-lint` was absent for all four tasks. CI runs it; nothing in this milestone was
  linted locally.
- **`safe-commit.sh` could not be run, for the fourth time.** The invocation was refused
  before it executed ("contains shell syntax that cannot be statically analyzed"), and
  `ls /home/ralph/code/ai-lawnmower/loop/safe-commit.sh` was refused outright — this
  session may only reach `/workspace`, so the script's existence still cannot be
  confirmed. F1 and the completion commit used PROMPT.md's "If safe-commit cannot be
  reached" fallback: one `git add <path>` per touched path, by name, then `git commit -m`.

## Findings carried forward, open

- **`internal/sessionpod` (seven cases) and `internal/loginrelay`
  (`TestCodexSignedIn/signed_out_on_stdout`) fail under a loaded `go test ./...` and pass
  when run alone.** Five-second deadlines and a write-then-exec of a fake binary; nothing
  this milestone touched is in either package. Not fixed here — out of this plan's "Files
  touched" — and recorded so the next verify run does not read it as this fix regressing.

## Completion

Every task in IMPLEMENTATION_PLAN.md is `- [x]`. `go test -count=1 ./...` exits 0,
`go vet -tags tmux ./...` exits 0, `go vet -tags quickstart ./cmd/crswd` exits 0.
`golangci-lint` is not installed in this container and was not run in any of the four
iterations.

RALPH_COMPLETE
