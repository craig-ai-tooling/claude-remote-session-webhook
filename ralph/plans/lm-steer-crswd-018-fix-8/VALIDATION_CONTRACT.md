# Validation contract: a Claude prompt is one bracketed paste plus one Enter (lm-steer-crswd-018-fix-8)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree, from the repository
root. Every assertion says how it is checked.

The failing assertion this milestone answers came from the parent milestone
lm-steer-crswd-018 (spec `specs/020-prompt-route-safety`, FR-009): "a two-line
prompt to a Claude session arrives as one bracketed paste followed by exactly one
Enter". The suite asserted the opposite for Claude.

## Behaviour of `POST /sessions/{id}/prompt` and the session layer behind it

- A two-line prompt (`"first line\nsecond line"`) delivered to a Claude session runs exactly three tmux commands: a load-buffer carrying the whole text on stdin, one `paste-buffer` whose argv carries `-p`, then one `send-keys … Enter`. Checked by `go test ./internal/session/ -run Prompt -v`, which exits 0 and prints at least one `--- PASS` line whose name contains `Prompt`.
- No test in the default suite asserts that a Claude prompt avoids bracketed paste: `go test ./internal/session/ -run 'Prompt' -v` exits 0, and `grep -rn 'a Claude prompt used bracketed paste' internal/` prints nothing.
- Against a real tmux, a two-line prompt into a pane whose program has requested bracketed-paste mode arrives as exactly one bracketed block (one start marker, one end marker, both lines inside it) and is submitted once: `go test -tags tmux ./internal/session/ -run Prompt -v` exits 0 and prints at least one `--- PASS` line whose name contains `Prompt`.
- A Codex prompt is still bracketed and still followed by one Enter: `go test ./internal/session/ -run PromptCodex` exits 0.
- Compact on a Claude session is unchanged: it is one plain paste of `/compact` plus newline and no `send-keys`: `go test ./internal/session/ -run Compact` exits 0.
- The prompt route still answers `202` with the text delivered, and still answers `500` with no tmux detail when the paste or the Enter fails: `go test ./internal/httpapi/ -run Prompt` exits 0.
- No audit record carries prompt text after the change, including when the host refuses the paste: `go test ./internal/audit/` exits 0.

## The suites and guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0.
- `go test -tags tmux ./...` exits 0 on a host with tmux installed.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0.
- `gofmt -l .` prints nothing.
- The fix is logged: `grep -q 'bracketed' docs/fixes-log.md` exits 0 and the last line of `docs/fixes-log.md` mentions a Claude prompt.
- No new package under `internal/`: `bash -c 'diff <(git ls-tree -d --name-only main internal/) <(git ls-tree -d --name-only HEAD internal/)'` exits 0 and prints nothing.
- No `go.sum` is created: `test ! -e go.sum` exits 0.
