# Implementation Plan: Weekly Quota Bar in the Header

**Branch**: `feat/weekly-quota-bar` | **Date**: 2026-09-14 | **Spec**: [spec.md](spec.md)

## Summary

A new `internal/quota` package reads quota-axi's own on-disk cache — no exec,
no network. A new route, `GET /dashboard/quota`, modelled on
`GET /dashboard/auth` (spec 015), answers a small JSON shape from it. The
header renders a `<meter>` and a text label below `.masthead-bar`, starting
"checking" exactly as the auth pill does, and `crswd.js` polls it on the same
terms the auth pill's own poll uses.

## Technical Context

**Language**: Go, standard library, server-rendered templates, one embedded
vanilla-JS asset. **Storage**: none new — quota-axi already wrote the file.
**Testing**: table-driven, `t.Parallel()`, fixture cache files under
`t.TempDir()`, never the operator's real `~/.cache`. No research.md or
data-model.md: the route's shape copies `GET /dashboard/auth`, the failure
discipline copies this tree's own "a check that cannot run refuses" rule, and
the cache path is quota-axi's `cacheDirPath()` transcribed, not reinvented.

## Constitution Check

| Principle | Assessment | Pass |
|---|---|---|
| **I — Security** | Read-only, behind the same identity every dashboard read requires, no page token minted; `internal/quota` execs nothing and opens no socket. | ✅ |
| **II/III — Unknowns surfaced, Verifiable** | Failure handling and thresholds were decided before this plan; every FR is a status code, a JSON shape, or a sentinel a test can assert. | ✅ |
| **IV — Smallest change** | New only where nothing else reads a sibling tool's cache; everything else mirrors `authstatus.go`. | ✅ |
| **V — Standards, VI — Blast radius** | The parity and route-secret sweeps gain this route rather than being relaxed around it; a session's environment and credential handling are untouched. | ✅ |
| **VII — Design system** | No new vocabulary beyond `.quota-bar`/`.quota-meter`/`.quota-label`; colour is existing state tokens. | ✅ |

## Design

`quota.Read(path)` parses quota-axi's JSON into a `Reading`, returning one of
six sentinels for anything short of a clean answer. `dashboardQuota` folds
every error into `state:"unknown"` and computes `stale` from `Reading.Stale`
OR a two-hour age check against `s.clock`, the clock every other cache here
uses. The `<meter>`'s `low`/`high`/`optimum` give the browser's own three fill
regions the 75%/90% thresholds; each vendor's pseudo-elements are overridden
to the same token pair so WebKit and Firefox render alike, with no script
computing a width or writing a `style` attribute the CSP forbids. The meter
ships `hidden` and the label carries `quota-label-unknown` until the first
fetch lands — the auth pill's own "starts as unknown" shape.

## Project Structure

```text
internal/quota/            NEW  quota.go, quota_test.go — the reader
internal/httpapi/
├── quotastatus.go          NEW  GET /dashboard/quota
├── quotastatus_test.go     NEW  handler + script sweep tests
└── server.go                MOD  quotaCachePath field; route registration
internal/audit/audit.go     MOD  ActionDashboardQuota
web/templates/partials/header.html  MOD  the bar
web/static/crswd.css                MOD  .quota-*, meter theming
web/static/crswd.js                 MOD  the poll module
docs/components.md                  MOD  Header: the weekly-quota bar
```
