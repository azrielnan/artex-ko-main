#!/usr/bin/env bash
set -euo pipefail

target="${1:-}"
destination="${2:-}"
node_version="${ARTEX_REA_NODE_VERSION:-22.19.0}"

die() { printf '[x] %s\n' "$*" >&2; exit 1; }
[ -n "$target" ] || die "usage: bundle-rea.sh OS/ARCH DESTINATION"
[ -n "$destination" ] || die "usage: bundle-rea.sh OS/ARCH DESTINATION"
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v npm >/dev/null 2>&1 || die "npm is required"

goos="${target%%/*}"
goarch="${target##*/}"
case "$target" in
  linux/amd64) node_platform=linux-x64; npm_os=linux; npm_cpu=x64 ;;
  linux/arm64) node_platform=linux-arm64; npm_os=linux; npm_cpu=arm64 ;;
  darwin/amd64) node_platform=darwin-x64; npm_os=darwin; npm_cpu=x64 ;;
  darwin/arm64) node_platform=darwin-arm64; npm_os=darwin; npm_cpu=arm64 ;;
  windows/amd64) node_platform=win-x64; npm_os=win32; npm_cpu=x64 ;;
  *) die "unsupported REA runtime target: $target" ;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$destination"
archive="node-v${node_version}-${node_platform}.$([ "$goos" = windows ] && printf zip || printf tar.xz)"
url="https://nodejs.org/dist/v${node_version}/${archive}"
curl --fail --location --retry 3 --silent --show-error "$url" --output "$tmp/$archive"

if [ "$goos" = windows ]; then
  command -v unzip >/dev/null 2>&1 || die "unzip is required to package Windows Node.js"
  unzip -q "$tmp/$archive" -d "$tmp/node"
  cp "$tmp/node/node-v${node_version}-${node_platform}/node.exe" "$destination/node.exe"
  cp "$tmp/node/node-v${node_version}-${node_platform}/LICENSE" "$destination/NODE-LICENSE.txt"
  cp -R "$tmp/node/node-v${node_version}-${node_platform}/LICENSES" "$destination/" 2>/dev/null || true
else
  tar -xJf "$tmp/$archive" -C "$tmp"
  cp "$tmp/node-v${node_version}-${node_platform}/bin/node" "$destination/node"
  chmod +x "$destination/node"
  cp "$tmp/node-v${node_version}-${node_platform}/LICENSE" "$destination/NODE-LICENSE.txt"
  cp -R "$tmp/node-v${node_version}-${node_platform}/LICENSES" "$destination/" 2>/dev/null || true
fi

npm_args=(install --prefix "$destination" --omit=dev --ignore-scripts --no-audit --no-fund --os="$npm_os" --cpu="$npm_cpu")
if [ "$npm_os" = linux ]; then npm_args+=(--libc=glibc); fi
npm "${npm_args[@]}" rea-agents@6.5.0

test -f "$destination/node_modules/rea-agents/scripts/rea.mjs" || die "REA entry point is missing after npm install"
test -f "$destination/node_modules/rea-agents/LICENSE" || die "REA license is missing from the installed package"
printf 'Bundled REA %s with Node.js %s for %s\n' "6.5.0" "$node_version" "$goos/$goarch"
