# Validation contract: the quickstart suite compiles under the module's go 1.23 floor (lm-steer-crswd-018-fix-9-fix-1)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history, no diff and no `.git` directory runs these against the tree.
All commands run from the repository root.

Run `1022-6xpl` (PR #234) failed the parent milestone's assertion
"`go vet -tags quickstart ./cmd/crswd` exits 0" with
`testing.Context requires go1.24 or later (file is go1.23)`. The module declares
`go 1.23.0`, so the quickstart suite must compile against that language version.

- The quickstart acceptance suite type-checks: `go vet -tags quickstart ./cmd/crswd` exits 0 and prints nothing.
- No tagged or untagged suite regressed: `go vet ./...`, `go vet -tags tmux ./...` and `go vet -tags dev ./...` each exit 0.
- The module's language floor is unchanged, so the fix did not come from raising it: `grep -x 'go 1.23.0' go.mod` prints `go 1.23.0` and exits 0.
- Nothing in the tracked Go sources of the main module uses the go1.24-only test context: `grep -rn --include=*_test.go 't\.Context()' cmd internal` prints nothing and exits 1.
- The touched package is gofmt-clean: `gofmt -l cmd/crswd` prints nothing.
- The product still builds and its default tests pass: `go build ./... && go test ./...` exits 0.
