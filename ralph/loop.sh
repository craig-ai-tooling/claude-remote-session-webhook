#!/usr/bin/env bash
# Ralph loop — run Claude Code headless, ONE task per iteration, fresh context.
#
# The whole point is a fresh context every iteration. Long-running sessions drift,
# forget the plan, and start inventing. Each iteration re-reads the plan from disk,
# does exactly one task, commits, and exits. State lives in git and in
# ralph/PROGRESS.md — never in a conversation.
#
# Usage:
#   ./ralph/loop.sh              # up to 20 iterations
#   ./ralph/loop.sh 5            # up to 5
#   DRY_RUN=1 ./ralph/loop.sh 1  # print the command, run nothing
#
# Stops when: the cap is hit, the plan reports complete, or an iteration fails.
set -euo pipefail

MAX_ITERATIONS="${1:-20}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ── which task's notebook? ──────────────────────────────────────────────
# One notebook per task, at ralph/plans/<id>/ (lm-guard-allowlist-single-file-
# conflicts, 9/16/26) -- 193 of 200 merged PRs used to rewrite the one shared
# ralph/IMPLEMENTATION_PLAN.md purely to declare their own allowlist, so every
# merge conflicted every other open PR against that same file.
#
# Resolution order: $RALPH_PLAN_DIR verbatim if set; else ralph/plans/$RALPH_PLAN
# if that is set; else, if the current branch is literally named plan/<id>
# (the shape loop/ralph-prepare.sh cuts and a human running this by hand on
# that branch would also be on), derive <id> from it. A pod running the loop
# is normally on ralph/<runid>, not plan/<id> -- job.yaml sets RALPH_PLAN on
# the ralph container for exactly that reason (ralph-runner/ralph-launch.sh).
if [[ -z "${RALPH_PLAN_DIR:-}" ]]; then
  PLAN_ID="${RALPH_PLAN:-}"
  if [[ -z "$PLAN_ID" ]]; then
    CUR_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")"
    if [[ "$CUR_BRANCH" =~ ^plan/([A-Za-z0-9._-]+)$ ]]; then
      PLAN_ID="${BASH_REMATCH[1]}"
    fi
  fi
  if [[ -z "$PLAN_ID" ]]; then
    echo "FATAL: cannot resolve which task's notebook to run." >&2
    echo "       Set RALPH_PLAN_DIR=ralph/plans/<id>, or RALPH_PLAN=<id>, or" >&2
    echo "       run this on a branch literally named plan/<id>." >&2
    exit 2
  fi
  # loop/plan-gate.sh and loop/verify-run.sh both refuse this same shape for
  # --plan; this path resolved it unchecked (RALPH_PLAN='../../elsewhere' read
  # a notebook entirely outside ralph/plans/, past the FATAL below, until this
  # guard was added -- review finding, 9/16/26). The branch-derived PLAN_ID
  # above is regex-constrained but that class still permits '..', so check
  # every source the same way.
  case "$PLAN_ID" in
    *..*|.*|*/*)
      echo "FATAL: invalid RALPH_PLAN '$PLAN_ID' (no '..', no leading '.', no '/')" >&2
      exit 2 ;;
  esac
  RALPH_PLAN_DIR="ralph/plans/${PLAN_ID}"
else
  # An explicit RALPH_PLAN_DIR is taken verbatim by design (it names the
  # notebook directly), but it must still resolve inside ralph/plans/ -- same
  # traversal guard as the PLAN_ID branch above.
  case "$RALPH_PLAN_DIR" in
    ralph/plans/*..*)
      echo "FATAL: invalid RALPH_PLAN_DIR '$RALPH_PLAN_DIR' (no '..')" >&2
      exit 2 ;;
    ralph/plans/*) ;;
    *)
      echo "FATAL: invalid RALPH_PLAN_DIR '$RALPH_PLAN_DIR' (must start with ralph/plans/)" >&2
      exit 2 ;;
  esac
fi

PROMPT_FILE="${RALPH_PLAN_DIR}/PROMPT.md"
PLAN_FILE="${RALPH_PLAN_DIR}/IMPLEMENTATION_PLAN.md"
PROGRESS_FILE="${RALPH_PLAN_DIR}/PROGRESS.md"
DONE_SIGNAL="RALPH_COMPLETE"

for f in "$PROMPT_FILE" "$PLAN_FILE" "$PROGRESS_FILE"; do
  [[ -f "$f" ]] || { echo "FATAL: missing $f" >&2; exit 1; }
done
command -v claude >/dev/null 2>&1 || { echo "FATAL: 'claude' CLI not on PATH" >&2; exit 1; }

if [[ -n "$(git status --porcelain)" ]]; then
  echo "FATAL: working tree is dirty. Commit or stash first — the loop commits" >&2
  echo "       after every iteration and must start from a clean base." >&2
  exit 1
fi

# The sentinel must not already be present, or the first iteration's check finds
# the *previous* milestone's completion and exits reporting this plan complete
# with every task still open.
#
# This has been live at the start of two milestones running. Both times it was
# caught by hand, and the second time it happened despite Iteration 0 of the
# first having written the warning down — because that note lives in the file
# that gets archived, so the reminder is archived along with the trap. A guard
# here cannot be archived.
if grep -qxF "$DONE_SIGNAL" "$PROGRESS_FILE" 2>/dev/null; then
  echo "FATAL: ${PROGRESS_FILE} already contains ${DONE_SIGNAL}." >&2
  echo "       That is this loop's exit sentinel, so iteration 1 would find it and" >&2
  echo "       stop, reporting the plan complete with every task still open." >&2
  echo "       Archive the finished milestone's notebook and start a fresh one." >&2
  exit 1
fi

BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$BRANCH" == "main" || "$BRANCH" == "master" ]]; then
  echo "FATAL: refusing to run on '$BRANCH'. Branch first:" >&2
  echo "       git switch -c feat/<name>" >&2
  exit 1
fi

echo "Ralph loop: max ${MAX_ITERATIONS} iterations on branch '${BRANCH}'"
echo

for (( i=1; i<=MAX_ITERATIONS; i++ )); do
  echo "──────────────────────────────────────────────────────────"
  echo "  Iteration ${i}/${MAX_ITERATIONS}  ($(date -u +%H:%M:%SZ))"
  echo "──────────────────────────────────────────────────────────"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    echo "[dry-run] claude -p \"\$(cat $PROMPT_FILE)\" --permission-mode acceptEdits"
    break
  fi

  # Fresh process = fresh context. Settings are passed explicitly so the repo
  # hooks (danger-guard, format-and-lint) apply to autonomous runs too.
  set +e
  # stdin is redirected from /dev/null: with no terminal attached (cron, nohup,
  # a background job), claude waits ~3s for stdin that never arrives and warns.
  # Harmless but noisy, and it makes real failures harder to spot in the log.
  claude -p "$(cat "$PROMPT_FILE")" \
    --permission-mode acceptEdits \
    --settings .claude/settings.json \
    < /dev/null
  rc=$?
  set -e

  if [[ $rc -ne 0 ]]; then
    echo
    echo "Iteration ${i} exited ${rc}. Stopping so a human can look." >&2
    exit "$rc"
  fi

  # The agent is instructed to commit its own work. If it left anything staged
  # or dirty, capture it rather than silently carrying it into the next context.
  if [[ -n "$(git status --porcelain)" ]]; then
    git add -A
    # No --no-verify. AGENTS.md lists the gitleaks pre-commit hook as enforced,
    # and an unattended sweep is precisely when you least want it bypassed —
    # nobody is reading this diff. If the hook rejects, stop rather than commit
    # unscanned work.
    if ! git commit -m "ralph: iteration ${i} (sweep uncommitted changes)"; then
      echo >&2
      echo "Sweep commit rejected by the pre-commit hook. Stopping so a human can look." >&2
      exit 1
    fi
    echo "  (swept uncommitted changes into a commit)"
  fi

  # -x -F: the signal must be a line that is EXACTLY the sentinel, which is what
  # PROMPT.md asks the agent to append. A bare substring grep also matches prose
  # that merely mentions the sentinel — PROGRESS.md documents it by name — so the
  # loop declared victory after one iteration with 41 tasks still open.
  if grep -qxF "$DONE_SIGNAL" "$PROGRESS_FILE" 2>/dev/null; then
    echo
    echo "Found ${DONE_SIGNAL} in ${PROGRESS_FILE}. Plan complete after ${i} iteration(s)."
    exit 0
  fi

  echo "  Iteration ${i} done."
  echo
done

echo "Reached the ${MAX_ITERATIONS}-iteration cap without ${DONE_SIGNAL}."
echo "Review ${PROGRESS_FILE}, then re-run to continue."
