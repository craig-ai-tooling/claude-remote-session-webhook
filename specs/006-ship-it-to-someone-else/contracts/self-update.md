# Contract: Self-update

**Files**: `internal/updater/{fetch,verify,stage,swap}.go`, `internal/httpapi/update.go`
**Tests**: `internal/updater/*_test.go`, `internal/httpapi/update_test.go`
**Satisfies**: FR-021 … FR-030
**Decomposed**: five tasks. See the bottom.

---

## The order is the security property

```
1. fetch     → ~/.local/share/crswd/staging/crswd.<version>   mode 0600
2. checksum  → sha256 of the bytes against SHA256SUMS
3. signature → SHA256SUMS.sig against the embedded ed25519 key
4. chmod     → 0700, the first moment it is executable at all
5. smoke     → exec it with --version; must print exactly <version>
6. rename    → atomically over ~/.local/bin/crswd
7. exit 0    → systemd restarts into it
```

**Any step failing means the daemon keeps running exactly what it was running**
(FR-028). Every failure here is a refusal, because refusing is always safe and
installing something unverified is not.

**Steps 2 and 3 are not interchangeable and 3 is not optional.** A checksum
travels with the thing it describes, so anyone who can serve you a binary can
serve you its checksum. The signature is what makes the checksum mean anything,
and it is verified against a key that was embedded when the operator installed
this binary — a decision made before the attacker was involved.

**Step 5 is the one that is not obvious.** A checksum proves the bytes are the
published bytes. It says nothing about whether they *run on this host*. An arm64
build on an amd64 machine passes 2 and 3 and then fails to exec — and by then it
would be the installed binary. Running it once, first, turns an outage into a
refusal.

## The route

| Property | Literal |
|---|---|
| Path | `POST /dashboard/update` |
| Registered via | `s.handleAction(...)`, the same call destroy uses |
| Confirming field | `confirm`, must equal `yes` — reuse `fieldConfirm`/`confirmYes` |
| Version field | `version`, optional; absent means latest |
| Audit action | `dashboard.update` |
| Cross-site | Both halves, unchanged, inherited from `handleAction` |

Registering through `handleAction` rather than by hand is the whole point: the
gate, the audit record and the ownership check are not re-implemented here, so
they cannot be re-implemented *differently* here.

## What must not happen

- **Nothing executable before step 3 completes.** Staged bytes are `0600`.
- **No redirect to another host.** The HTTP client refuses a redirect whose host
  differs from the expected one (FR-026).
- **No asset by pattern.** The asset is named exactly; "ends in `_arm64.tar.gz`"
  is a looser check than FR-027 asks for.
- **No major crossed, ever, blank or named** (k8s-18). An empty `version`
  resolves to the newest non-prerelease release sharing this build's own major
  (`internal/buildinfo.Version`) — never GitHub's `latest` pointer. `latest` is
  whatever is newest across every major, so the day a v2 release is marked
  latest, a blank ask would otherwise install it over a v0 production daemon
  with nothing about the request suggesting a refusal (Craig, 9/14/26: "what I
  don't want to do is accidentally update crswd"). A named version is held to
  the identical boundary: one whose major does not match the running build's
  is refused, naming both majors, before anything is asked of GitHub. A build
  that cannot read its own version — an unstamped `dev` build most of all —
  refuses either kind of ask outright rather than guess which major is safe.
- **The signing key never appears** in a log, an audit record, or a page. The
  daemon holds only the public half, and even that is never rendered.
- **A staged file never survives a restart.** The staging directory is swept at
  startup: the process that vouched for those bytes did not live to say so.

## Worked example

Daemon on `v0.41`, `v0.42` published:

```
POST /dashboard/update
confirm=yes
→ 303, audit dashboard.update allow
→ stages crswd.v0.42 (0600), checksum ok, signature ok, chmod 0700
→ exec --version prints "crswd v0.42" ✓
→ rename over ~/.local/bin/crswd, keep previous at crswd.previous
→ exit 0; systemd restarts; GET /dashboard/version now says v0.42
```

Same request, one byte of the tarball corrupted:

```
→ stages (0600), checksum FAILS
→ staged file removed, nothing renamed, daemon still on v0.41
→ audit dashboard.update deny
```

Rollback is the same route with `version=v0.41`.

Daemon on `v0.100`, a `v2.0.0` release published and marked `latest`, `v0.118`
also published (k8s-18):

```
POST /dashboard/update
confirm=yes
→ resolves the blank ask against the release list, not `latest`
→ v2.0.0 is a different major and is skipped; v0.118 is the newest release
  sharing this build's major
→ stages crswd.v0.118, same as any other update
```

Same daemon, `version=v2.0.0` named explicitly:

```
POST /dashboard/update
confirm=yes
version=v2.0.0
→ refused before GitHub is asked anything: "v2.0.0 is major 2; this build
  (v0.100) is major 0"
→ nothing staged, nothing renamed, daemon still on v0.100
→ audit dashboard.update deny
```

## Contract tests

| Test | Asserts | **Must fail when** |
|---|---|---|
| `TestStagedFileIsNotExecutableBeforeVerification` | Mode is `0600` until both checks pass | Anything is chmod'd executable before the signature verifies |
| `TestChecksumMismatchRefuses` | Update refused, nothing renamed, version unchanged | A mismatch is logged and the update proceeds |
| `TestSignatureMismatchRefuses` | Refused | Only the checksum is checked — the failure that makes signing decorative |
| `TestUnsignedReleaseRefuses` | A release with no `.sig` is refused | Absence is treated as "nothing to verify against" rather than as a refusal |
| `TestSmokeTestCatchesWrongArchitecture` | A staged binary that cannot exec is refused before the rename | The smoke test is skipped, so a cryptographically perfect wrong-arch binary becomes the installed one |
| `TestSmokeTestRequiresMatchingVersion` | A binary printing a different version is refused | It only checks exit status, so any runnable file passes |
| `TestCrossHostRedirectRefused` | A redirect to another host is refused | The default client is used, which follows redirects anywhere |
| `TestAssetMatchedByExactName` | A differently-named asset is refused | Suffix matching is used |
| `TestFailedUpdateLeavesNothingExecutable` | After any failure, the staging dir holds nothing executable | A partial download is left behind at 0700 |
| `TestStagingSweptAtStartup` | A staged file present at boot is removed, not used | It is trusted across a restart |
| `TestPreviousBinaryKept` | `crswd.previous` exists after a successful swap | Rollback has nothing to roll back to |
| `TestUpdateRequiresConfirm` | `confirm` absent or ≠ `yes` → refused | The confirming step is dropped from a route that replaces the running binary |
| `TestUpdateCrossSiteBothHalves` | Missing `Sec-Fetch-Site` and a bad page token each refuse independently | A test disables either half instead of satisfying it (AR-005) |
| `TestUpdateEmitsExactlyOneAuditRecord` | One record, action `dashboard.update` | Each stage emits its own |
| `TestNoKeyMaterialInAnyOutput` | No response, log or record contains the key | A diagnostic prints what it verified against |
| `TestBlankAskResolvesNewestSameMajorNeverLatest` (k8s-18) | A blank ask installs the newest non-prerelease release sharing this build's major, with a `latest`-marked release of a different major present | `latest` is trusted, or any release outside the running major outranks one sharing it |
| `TestNamedVersionAcrossMajorRefused` (k8s-18) | A named version whose major differs from the running build's is refused, naming both majors, before the network is touched | Only the version's shape is checked and not its major |
| `TestUnknownRunningVersionRefusesUpdate` (k8s-18) | A build whose own version cannot be read (`dev`) refuses any update, blank or named | An unparseable running version is treated as "no constraint" |

## The five tasks

1. **fetch** — TLS, exact asset name, no cross-host redirect. No verification yet.
2. **verify** — sha256 then ed25519, in that order, against the embedded key.
3. **stage** — the `0600` invariant, the startup sweep, cleanup on failure.
4. **swap** — smoke test, atomic rename, keep previous, exit.
5. **route** — `handleAction`, the confirming step, the audit record.

Task 1 must leave a daemon that can download and verify **nothing installed** —
if it can swap, verification is being built after the thing it protects.
