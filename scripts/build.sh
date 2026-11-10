#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
npm --prefix web ci
npm --prefix web run build
version=$(tr -d '\r\n' < VERSION)
target_os="${GOOS:-$(go env GOOS)}"
target_arch="${GOARCH:-$(go env GOARCH)}"
suffix=""
if [ "$target_os" = "windows" ]; then suffix=".exe"; fi
mkdir -p dist
CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -buildvcs=false \
  -ldflags "-s -w -X main.version=$version -X main.commit=${GITHUB_SHA:-dev} -X main.updateKey=${CABBAGE_UPDATE_PUBLIC_KEY:-}" \
  -o "dist/cabbage-$target_os-$target_arch$suffix" ./cmd/cabbage
