#!/usr/bin/env bash
# Fetch the pinned Codex binary into <dir>/codex, for the session image's build
# context (spec 017 FR-021). The archive is checked against a digest measured from
# the release, so a swapped asset fails the build instead of shipping.
#
# Usage: fetch-codex.sh <dir>
set -euo pipefail

CODEX_VERSION=0.153.4
ASSET=codex-x86_64-unknown-linux-musl.tar.gz
SHA256=f479424eca092484dc40d87ae28c44f4cc40234a60045d6131e493800d814a30
MEMBER=codex-x86_64-unknown-linux-musl

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
	echo "usage: fetch-codex.sh <dir>" >&2
	exit 2
fi
dir=$1

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
archive=$work/$ASSET

curl -fsSL -o "$archive" "https://github.com/openai/codex/releases/download/rust-v${CODEX_VERSION}/${ASSET}"
echo "$SHA256  $archive" | sha256sum -c - >/dev/null

mkdir -p "$dir"
tar -xzf "$archive" -C "$work" "$MEMBER"
install -m 0755 "$work/$MEMBER" "$dir/codex"
