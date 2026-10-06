# The crswd image: the daemon and the reconciler (spec 017, k8s-20c slice S7).
#
# One binary, two Deployments. The daemon runs it with no arguments and the
# reconciler runs it as `crswd reconcile`. It is the cluster build
# (k8s/cmd/crswd), which refuses a host configuration; the host install keeps
# install.sh and the release binary.
#
# TAG. docker.io/nctiggy/crswd:2.0.0-dev.<sha7>
#   <sha7>   the crswd commit the binary was built from (git rev-parse --short=7).
# The cluster has no pull secret for a private registry, so this is Docker Hub.
# Never push a tag twice: pods use IfNotPresent, and a node keeps the layer it
# already has under a name you have since reused.
#
# PLATFORM. linux/amd64 and linux/arm64. The binary is static (CGO_ENABLED=0), so
# the distroless static base is enough. TARGETARCH is set by buildx for each
# platform and picks the matching file from the build context.
#
# BUILD. The context is a directory holding just the two binaries:
#
#   BIN=$(mktemp -d)
#   for a in amd64 arm64; do CGO_ENABLED=0 GOOS=linux GOARCH=$a go -C k8s build -trimpath -ldflags "-X main.version=2.0.0-dev.$(git rev-parse --short=7 HEAD)" -o "$BIN/crswd-$a" ./cmd/crswd; done
#   docker buildx build --platform linux/amd64,linux/arm64 -f deploy/image/Dockerfile -t docker.io/nctiggy/crswd:2.0.0-dev.$(git rev-parse --short=7 HEAD) --push "$BIN"
#
# CREDENTIALS. None are baked in, and the build takes none: no ENV, no secret
# mount. The shared secret arrives from a Secret at run time.
# gcr.io/distroless/static-debian12:nonroot
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
ARG TARGETARCH

# COPY and not ADD: ADD fetches URLs and unpacks archives, and this image has no
# business doing either. The binary is root-owned and not writable by the process.
COPY --chmod=0755 crswd-${TARGETARCH} /usr/local/bin/crswd

# Numeric, so that runAsNonRoot can be checked by the kubelet, which cannot
# resolve a name to a uid. This is the uid the nonroot base already uses.
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/crswd"]
