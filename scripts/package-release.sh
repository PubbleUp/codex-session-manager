#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION_FILE="$ROOT_DIR/internal/version/version.go"
OUTPUT_ROOT="$ROOT_DIR/bin/release"
PACKAGE_NAME="codex-session-manager"

if [[ $# -gt 1 ]]; then
  printf '用法：%s [版本号]\n' "$0" >&2
  exit 2
fi

if [[ $# -eq 1 ]]; then
  VERSION="$1"
else
  VERSION="$(sed -n 's/^var Version = "\([^"]*\)"$/\1/p' "$VERSION_FILE")"
fi

if [[ -z "$VERSION" || ! "$VERSION" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  printf '无效版本号：%s\n' "${VERSION:-<空>}" >&2
  exit 2
fi

VERSION="${VERSION#v}"
OUTPUT_DIR="$OUTPUT_ROOT/$VERSION"
mkdir -p "$OUTPUT_ROOT"
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

cd "$ROOT_DIR"

declare -a TARGETS=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
  "windows arm64"
)

for target in "${TARGETS[@]}"; do
  read -r GOOS GOARCH <<< "$target"
  suffix=""
  if [[ "$GOOS" == "windows" ]]; then
    suffix=".exe"
  fi
  filename="${PACKAGE_NAME}_${VERSION}_${GOOS}_${GOARCH}${suffix}"
  printf '构建 %s/%s -> %s\n' "$GOOS" "$GOARCH" "$filename"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build \
    -trimpath \
    -ldflags "-s -w -X github.com/sunlock/codex-session-manager/internal/version.Version=$VERSION" \
    -o "$OUTPUT_DIR/$filename" \
    ./cmd/codex-session-manager
done

if command -v shasum >/dev/null 2>&1; then
  (cd "$OUTPUT_DIR" && shasum -a 256 "$PACKAGE_NAME"_* > SHA256SUMS.txt)
elif command -v sha256sum >/dev/null 2>&1; then
  (cd "$OUTPUT_DIR" && sha256sum "$PACKAGE_NAME"_* > SHA256SUMS.txt)
else
  printf '未找到 shasum 或 sha256sum，无法生成 SHA256SUMS.txt\n' >&2
  exit 1
fi

printf '\n生产包已生成：%s\n' "$OUTPUT_DIR"
printf '校验文件：%s/SHA256SUMS.txt\n' "$OUTPUT_DIR"
