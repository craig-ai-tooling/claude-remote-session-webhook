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
