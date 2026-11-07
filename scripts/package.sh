#!/bin/sh
set -eu

version=${1:-dev}
case "$version" in
  ''|*[!a-zA-Z0-9._-]*)
    printf '%s\n' 'Version must contain only letters, numbers, dots, underscores, or hyphens.' >&2
    exit 1
    ;;
esac

cd "$(dirname "$0")/.."
mkdir -p dist
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT HUP INT TERM

for arch in amd64 arm64; do
  name="keydebounce_${version}_linux_${arch}"
  mkdir -p "$stage/$name/packaging/systemd"
  GOOS=linux GOARCH="$arch" CGO_ENABLED=0 "${GO:-go}" build -trimpath \
    -o "$stage/$name/keydebounce" ./cmd/keydebounce
  cp LICENSE README.md CONTRIBUTING.md "$stage/$name/"
  cp -R docs "$stage/$name/"
  cp packaging/systemd/keydebounce.service "$stage/$name/packaging/systemd/"
  tar -czf "dist/$name.tar.gz" -C "$stage" "$name"
done

(cd dist && sha256sum "keydebounce_${version}_linux_amd64.tar.gz" \
  "keydebounce_${version}_linux_arm64.tar.gz" > "checksums_${version}.txt")
