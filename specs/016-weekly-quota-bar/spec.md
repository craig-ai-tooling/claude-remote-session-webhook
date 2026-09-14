# Feature Specification: Weekly Quota Bar in the Header

**Feature Branch**: `feat/weekly-quota-bar`
**Created**: 2026-09-14
**Status**: Draft
**Input**: Operator: "update crswd to have a weekly quota progress bar at the
top? I just want to see total usage at the weekly level." Data source, route
shape, failure handling and thresholds were decided before this spec rather
than left `NEEDS CLARIFICATION`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - The header always says total weekly usage (Priority: P1)

An operator glances at any page and sees how much of this week's Claude quota
is used, without opening quota-axi separately.

**Independent Test**: Load any page while quota-axi's cache holds a claude
`seven_day` window; confirm the bar reads its `percentUsed` within 60s.

**Acceptance Scenarios**:

1. **Given** any page, **When** it renders, **Then** the header carries a
   `<meter>` and a text label below the masthead bar.
2. **Given** the cache holds a claude `seven_day` window, **When** the bar
   polls, **Then** it shows that percentage and the reset time, converted to
   the viewer's local time.
3. **Given** the cache is missing, unreadable, malformed, has no claude
   provider, no `seven_day` window, or an out-of-range percentage, **When**
   the bar polls, **Then** it reads "weekly quota: unknown" — never 0%, never
   a bar that looks like a real reading.
4. **Given** a page that has just loaded, **When** it renders, **Then** the
   bar reads "weekly quota: checking" before any script runs.
5. **Given** a reading older than two hours, or one quota-axi marks stale,
   **When** the bar polls, **Then** it still shows the number and says how
   old it is.

**Edge cases**: quota-axi never having run renders the same as any other
missing-cache case; a real 0% is a pointer field the response carries and
never collapses into the "unknown" omission; two open tabs each poll
independently since a read costs a stat and a small file.

## Requirements *(mandatory)*

- **FR-001**: Every page MUST carry a `<meter min="0" max="100">` and a text
  label below `.masthead-bar`, in the header partial.
- **FR-002**: `GET /dashboard/quota` MUST answer JSON
  `{"state":"ok"|"unknown","percentUsed":N,"resetsAt":"…","refreshedAt":"…","stale":bool}`,
  omitting the four value fields when unknown.
- **FR-003**: The route MUST require the same identity every dashboard read
  requires, register GET only, and answer `Cache-Control: no-store`.
- **FR-004**: A new `internal/quota` package MUST read quota-axi's own cache
  file — no exec, no network, no credential file — resolving its path exactly
  as quota-axi's `cacheDirPath()` does.
- **FR-005**: Every failure in Scenario 3 MUST be a sentinel checked with
  `errors.Is`, never a zero `Reading`.
- **FR-006**: `stale` MUST be true when quota-axi's `state.stale` is true OR
  `refreshedAt` (falling back to `generatedAt`) is over two hours old by the
  server's own clock.
- **FR-007**: The client MUST fetch `/dashboard/quota` on load and every 60s
  with `credentials: 'same-origin'`, `cache: 'no-store'`; a failed fetch MUST
  render "unknown".
- **FR-008**: The daemon MUST send `resetsAt`/`refreshedAt` verbatim as
  quota-axi wrote them; the browser converts to local time.
- **FR-009**: State MUST never be conveyed by colour alone.

**Key Entities**: **Reading** — `PercentUsed`, `ResetsAt`, `RefreshedAt`,
`Stale`, the claude provider's `seven_day` window and nothing else the cache
holds.

## Success Criteria *(mandatory)*

- **SC-001**: Every page carries the bar; none composes its own copy.
- **SC-002**: Every failure mode in FR-005 is a table-driven test asserting
  the specific sentinel, not merely a non-nil error.
- **SC-003**: A handler test fails when the route's registration is disabled.

**Assumptions**: no new configuration key — the path is derived the way
quota-axi derives it; a cache read is cheap enough to repeat per request,
unlike the auth pill's exec-backed cache.
