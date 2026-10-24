# Implementation plan: every lawnmower payload gives 202 with delivered:true (lm-steer-crswd-018-fix-13)

Fix lane, filed from a verify-run FAIL on parent milestone `lm-steer-crswd-018`
(run 1014-la1i). The parent's task T011 was never done: no fixture files exist
under `internal/httpapi/testdata/lawnmower/` and no test name contains `Lawnmower`,
so `go test ./internal/httpapi/ -run Lawnmower` prints `[no tests to run]`.
This milestone adds the four fixtures and the test. It changes no handler code.

## Facts measured at planning time (10/24/26, main `ebefd36`)

- `go test ./internal/httpapi/ -run Lawnmower` prints `ok ... [no tests to run]` and exits 0.
- `internal/httpapi/testdata/` holds one file, `header_no_codex.golden.html`. No `lawnmower/` directory.
- The fixture texts are in `ralph/plans/lm-steer-crswd-018/PROGRESS.md`, section
  `## Payload fixtures (for T011)` (lines 42 to 112). Each file is introduced by a line like
  `` `answer.txt`: ``, a blank line, then a ``` fence. Extracted byte for byte (every line ending
  in `\n`, fence lines excluded) they measure:

  | File | Bytes | Lines | First characters |
  |---|---|---|---|
  | `answer.txt` | 605 | 9 | `Craig answer` |
  | `resume.txt` | 622 | 11 | `Craig has pu` |
  | `opp-lease-batch.txt` | 730 | 14 | `2 more inbox` |
  | `opp-followup.txt` | 461 | 8 | `Another inbo` |

- None starts with `!` or `/`, so the leading-command refusal (fix-9, merged) does not refuse them.
  `resume.txt` has a later line `/compact only if the context gets long`; that is ordinary text.
- The prompt handler is `func (s *Server) promptSession` in `internal/httpapi/sessions.go` (line 779).
  It answers `202` with `promptResponse{ID, Delivered: true, ...}`.
- Prompt test helpers, all in package `httpapi`, in `internal/httpapi/sessions_test.go`:
  - `newPromptFixture(t)` (line 883) builds an audited server with one live session; `f.live.ID` is its ID.
  - `f.post(t, body, at)` (line 899) signs and posts; returns `(*httptest.ResponseRecorder, map[string]any)`.
  - `f.fixture.tmux.Calls()` returns `[]tmuxctl.Call`; the payload is on the one call whose `Stdin` is non-nil.
  - `promptRequest{Text: ...}` marshalled with `json.Marshal` is the body. `TestAPromptIsDeliveredByteForByte`
    (line 1052) is the model.
  - The replay cache is per server. A fresh `newPromptFixture(t)` per subtest means every subtest may use `testTime`.
- Sibling milestones may change the exact tmux call list (bracketed paste, liveness `List` calls,
  `request_id` in the 202). The new test must not assert the full call list or the number of response keys.
- `os.ReadFile` on a joined path trips gosec G304. Repo precedent (`internal/httpapi/dashboard_test.go:2070`):
  `//nolint:gosec // G304: the path is this helper's own literal, joined under testdata.`
- `golangci-lint` is at `/home/nctiggy/bin/golangci-lint` on the planning host. Earlier loops
  recorded it missing inside the loop container.
- `~/code/ai-lawnmower/loop/safe-commit.sh` exists on the planning host. Usage:
  `safe-commit.sh <repo> -m "message" -- path/one path/two`. Exit 0 committed, 1 checks failed, 2 usage.

## Decisions already made (do not reopen them)

- **K1 Fixture source.** The four files are copied byte for byte from the parent PROGRESS.md section
  above. Do not retype them. Use the extraction script below. If the shell refuses the heredoc,
  copy each block with the Write tool, one file per block, ending with a single `\n`.
- **K2 Test.** One new file `internal/httpapi/lawnmower_payload_test.go`, one test
  `TestLawnmowerPayloadsAreDelivered`, table-driven over the names `answer`, `resume`,
  `opp-lease-batch`, `opp-followup` (subtest names exactly these). `t.Parallel()` at both levels.
  A fresh `newPromptFixture(t)` per subtest. A helper reads `testdata/lawnmower/<name>.txt`
  with `os.ReadFile(filepath.Join("testdata", "lawnmower", name+".txt"))` under the G304 nolint
  comment above, and calls `t.Fatalf` if the read fails.
- **K3 Assertions per subtest.** Status is `202`. `body["delivered"] == true`. `body["id"] == f.live.ID`.
  Exactly one entry of `f.fixture.tmux.Calls()` has non-nil `Stdin`, and `bytes.Equal` holds between
  it and the file's bytes. No `Argv` element contains the file's text. Nothing else.
- **K4 No code change.** `internal/httpapi/sessions.go`, `internal/session/` and `internal/tmuxctl/`
  are untouched. If the test fails against the current handler, that is a finding: follow PROMPT.md
  "If the task is unclear or cannot be finished". Do not change the handler.
- **K5 Out of scope.** The parent plan's T011 box and `specs/020-prompt-route-safety/tasks.md`.

### Extraction script (T1)

Run from the repository root:

```bash
python3 - <<'EOF'
import os, re
src = open('ralph/plans/lm-steer-crswd-018/PROGRESS.md').read()
sec = src.split('## Payload fixtures (for T011)')[1].split('## Failing-before')[0]
os.makedirs('internal/httpapi/testdata/lawnmower', exist_ok=True)
n = 0
for m in re.finditer(r'`([a-z-]+\.txt)`:\n\n```\n(.*?)```\n', sec, re.S):
    with open('internal/httpapi/testdata/lawnmower/' + m.group(1), 'w', newline='') as f:
        f.write(m.group(2))
    n += 1
print(n, 'files written')
EOF
```

It prints `4 files written`.

## Pre-commit commands (every task, in this order)

1. `gofmt -l .` prints nothing.
2. `go build ./...` exits 0.
3. `go vet ./...` exits 0.
4. `go vet -tags tmux ./...` exits 0.
5. `go vet -tags quickstart ./cmd/crswd` exits 0.
6. `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` exits 0. If golangci-lint is not installed, record that in PROGRESS.md.
7. `test ! -e go.sum` exits 0.
8. `git status --short` lists only paths in "Files touched" or this notebook directory.

## Tasks

- [ ] T1 testdata: run the extraction script under K1 to write the four files in `internal/httpapi/testdata/lawnmower/`. Verify: `wc -c internal/httpapi/testdata/lawnmower/*.txt` prints 605 answer.txt, 461 opp-followup.txt, 730 opp-lease-batch.txt, 622 resume.txt, and `grep -c '^/compact' internal/httpapi/testdata/lawnmower/resume.txt` prints 1.
- [ ] T2 httpapi test: create `internal/httpapi/lawnmower_payload_test.go` with `TestLawnmowerPayloadsAreDelivered` per K2 and K3. Then do PROMPT.md "Prove the new test bites". Verify: `go test ./internal/httpapi/ -run Lawnmower -v` exits 0 and prints 4 `--- PASS: TestLawnmowerPayloadsAreDelivered/` lines.
- [ ] T3 docs: append one line dated 2026-10-24 to `docs/fixes-log.md` in its existing format: the four lawnmower prompt payloads (answer, resume, opp-lease batch, opp follow-up) are now fixtures and a test proves each gets 202 delivered. End with `(lm-steer-crswd-018-fix-13)`. Verify: `grep -n 'lm-steer-crswd-018-fix-13' docs/fixes-log.md | grep -q 2026-10-24` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go vet -tags tmux ./... && go vet -tags quickstart ./cmd/crswd` exits 0.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-13/` is always in scope):

```allowlist
internal/httpapi/lawnmower_payload_test.go
internal/httpapi/testdata/lawnmower/answer.txt
internal/httpapi/testdata/lawnmower/resume.txt
internal/httpapi/testdata/lawnmower/opp-lease-batch.txt
internal/httpapi/testdata/lawnmower/opp-followup.txt
docs/fixes-log.md
```

Not in scope, ever, in this milestone: `AGENTS.md`, `k8s/`, `specs/`, `internal/session/`,
`internal/tmuxctl/`, `internal/httpapi/sessions.go`, `internal/httpapi/sessions_test.go`, `go.mod`,
`go.sum`, `ralph/plans/lm-steer-crswd-018/`, any new directory under `internal/` other than
`internal/httpapi/testdata/lawnmower/`.
