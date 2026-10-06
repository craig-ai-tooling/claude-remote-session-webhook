# Validation contract — interactive input (spec 018)

What "done" means, stated as behaviour and checked from outside the code.

- A two-line message sent from the session page arrives in a real tmux pane wrapped in bracketed-paste markers, so the program sees one message and not two, verified by `go test -tags tmux ./internal/tmuxctl -run Bracketed`.
- A session created by the daemon keeps 5000 lines of history, and the scrollback read returns at most 5000 lines that exclude the visible screen, verified by `go test -tags tmux ./internal/tmuxctl -run 'FiveThousand|History'`.
- Posting text with enter=yes to the type route returns 204 with an empty body, and the session receives the text followed by one Enter, verified by `go test ./internal/httpapi -run TestTypeDeliversAndAnswersNoContent`.
- Posting text that holds an escape byte, is empty, or exceeds 16384 bytes delivers nothing and returns 303 to an outcome that names the reason, verified by `go test ./internal/httpapi -run TestTypeRefusesBadTextWithItsOutcome`.
- Every one of the twelve offered keys returns 204 and reaches the pane, and any other key name delivers nothing, verified by `go test ./internal/httpapi -run 'TestKeySendsEachAllowlistedKey|TestKeyRefusesAnythingElse'`.
- A cross-site request, a missing page token, or another operator's session is refused on both input routes with the same uniform answer every other action gives, verified by `go test ./internal/httpapi -run 'RefusesLikeEveryAction'`.
- After 120 rapid inputs, the next one delivers nothing and says the operator is going too fast, verified by `go test ./internal/httpapi -run TestKeyAndTypeShareOneBudget`.
- Typed text never appears in the audit trail or the logs, verified by `go test ./internal/audit -run Leak`.
- The scrollback is fetched only when opened, and is rendered as text, never markup, verified by `go test ./internal/httpapi -run 'Scrollback|TestTheSessionPageOffersTypingKeysAndScrollback'`.
- The API door's prompt route still delivers exactly as before, verified by `go test ./internal/session -run TestPromptPastesThenSubmits` passing unmodified.
