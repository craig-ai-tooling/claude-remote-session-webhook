# The image a kubernetes-mode session pod runs (spec 017, k8s-20c slice S3).
#
# One pod is one agent session: tmux and Claude Code from the base image, Codex
# beside it (FR-021), and
# the kubernetes module's crswd binary, which the reconciler starts as
# `crswd session-pod` (the tmux session) and the daemon reaches through the exec
# API as `crswd pane-loop` (FR-011). Both entrypoints are dispatched from that
# module (k8s/cmd/crswd, slice S7). The host daemon's binary is not this one and
# is not changed by this image.
#
# TAG. docker.io/nctiggy/crswd-session:2.1.246-codex0.153.4-<sha7>
#   2.1.246  the Claude Code version in the base image, so the tag says what a
#            session runs.
#   codex0.153.4  the Codex version fetch-codex.sh pins.
#   <sha7>   the crswd commit the binary was built from (git rev-parse --short=7).
# The cluster has no pull secret for a private registry, so this is Docker Hub.
# Never push a tag twice: pods use IfNotPresent, and a node keeps the layer it
# already has under a name you have since reused.
#
# PLATFORM. linux/amd64 only. Session pods are scheduled on lm-amd64-1, and the
# base image's amd64 slice is the one that carries claude 2.1.246 and tmux 3.5a
# (research D8a, D8b). The platform is pinned on the FROM line so a build on an
# arm64 machine fails instead of producing an image that does not run.
#
# BUILD. The COPY sources are files named crswd and codex at the root of the build
# context, so the context is a directory holding just those two and nothing else:
#
#   BIN=$(mktemp -d)
#   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go -C k8s build -trimpath -o "$BIN/crswd" ./cmd/crswd
#   deploy/session-image/fetch-codex.sh "$BIN"
#   docker build --network=host -f deploy/session-image/Dockerfile \
#     -t docker.io/nctiggy/crswd-session:2.1.246-codex0.153.4-$(git rev-parse --short=7 HEAD) "$BIN"
#
# CODEX. The CA bundle the Codex binary needs (spec 019 research M21) is already
# in the base image (ca-certificates 20250419, checked 10/6/26), so no RUN is added.
#
# CREDENTIALS. None are baked in, and the build takes none: no ARG, no ENV, no
# secret mount. The pod gets an access-token-only Secret mounted as a directory
# at run time (FR-015), and the reconciler names it in the pod spec without ever
# reading it.
# docker.io/nctiggy/ralph-runner:2.1.246-ci10
FROM --platform=linux/amd64 docker.io/nctiggy/ralph-runner:2.1.246-ci10@sha256:5986748b2442efdee9e23a96ce4351b317f8953676d7b750a290d03efdb47ad7

# COPY and not ADD: ADD fetches URLs and unpacks archives, and this image has no
# business doing either. The binary is root-owned and not writable by the session
# user, which is running code the operator did not write.
COPY --chmod=0755 crswd /usr/local/bin/crswd
COPY --chmod=0755 codex /usr/local/bin/codex

# The base image already has uid 10001 (user "ralph"), and the reconciler runs the
# pod as uid and fsGroup 10001 (FR-010). Numeric so that runAsNonRoot can be
# checked by the kubelet, which cannot resolve a name to a uid.
USER 10001:10001

# Where the claim's work directory is mounted. The pod's Run resolves the
# session's directory under it and refuses anything that escapes.
WORKDIR /work

# tini is already in the base image, and it matters here: tmux forks, and
# something has to reap what the session leaves behind.
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/crswd"]

# The base image's CMD starts the ralph loop. Cleared, so the arguments come from
# the pod spec and nothing else, and this image never runs that loop by default.
CMD []
