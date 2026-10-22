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
