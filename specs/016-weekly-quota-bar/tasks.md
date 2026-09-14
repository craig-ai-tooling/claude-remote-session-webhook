---
description: "Task list for 016-weekly-quota-bar"
---

# Tasks: Weekly Quota Bar in the Header

**Tests**: REQUIRED per `AGENTS.md`.

## Phase 1: The reader
- [X] T001 Add `internal/quota` (`Read`, `DefaultCachePath`, six sentinels, `Reading`).
- [X] T002 Tests: every failure sentinel, the ok path, the `generatedAt` fallback, both `DefaultCachePath` branches.

## Phase 2: The route
- [X] T003 Add `internal/httpapi/quotastatus.go` (`patternDashboardQuota`, `quotaStatusResponse`, `dashboardQuota`), modelled on `authstatus.go`.
- [X] T004 Add `Server.quotaCachePath`, resolved once in `newServer`; register the route behind `handleBrowser` with a new `audit.ActionDashboardQuota`.
- [X] T005 Tests: every reader failure folds to `unknown`, the ok path's fields, stale by flag, stale by age, fresh within the bound, no-store, requires identity, GET-only.

## Phase 3: The header and client
- [X] T006 Add the `<meter>` + label to `web/templates/partials/header.html`, below `.masthead-bar`.
- [X] T007 Add `.quota-bar`/`.quota-meter`/`.quota-label`/`.quota-label-unknown` to `web/static/crswd.css`, themed from existing state tokens via the meter's native fill regions.
- [X] T008 Add the poll module to `web/static/crswd.js`: fetch on load and every 60s, same-origin, no-store.
- [X] T009 Tests: the placeholder renders hidden/unknown, `TestHeaderHasExactlyTwoAnchors` unchanged, the script fetches the route on the right terms and repaints both the meter and the label.

## Phase 4: Polish
- [X] T010 Add a concise Header section to `docs/components.md` for the bar.
- [X] T011 Confirm a handler test fails with the route's registration disabled, then restore it.
- [X] T012 Run the full gate: `gofmt`, `go build`, `go vet` (default/tmux/quickstart), `go test` (default/dev), `golangci-lint`.
