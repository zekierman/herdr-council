#!/bin/sh
# Install step for macOS and Linux: download the prebuilt Council binary for this plugin version
# and verify its checksum; if that fails for any reason, build from source with Go.
set -eu
repo="zekierman/herdr-council"
root="$(cd "$(dirname "$0")/.." && pwd)"
version="$(sed -n 's/^version *= *"\(.*\)"/\1/p' "$root/herdr-plugin.toml")"
out="$root/bin/council.exe" # same name on every platform so the manifest needs one pane command
mkdir -p "$root/bin"

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) os="" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) arch="" ;;
esac

build() {
  echo "herdr-council: $1; building from source instead." >&2
  if ! command -v go >/dev/null 2>&1; then
    echo "herdr-council needs a prebuilt release for v$version or Go from https://go.dev/dl" >&2
    exit 1
  fi
  cd "$root" && CGO_ENABLED=0 go build -trimpath -o "$out" .
  exit 0
}

fetch() { # url dest
  if command -v curl >/dev/null 2>&1; then curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then wget -qO "$2" "$1"
  else return 1
  fi
}

[ -n "$os" ] && [ -n "$arch" ] || build "no prebuilt binary for $(uname -s)/$(uname -m)"
asset="council-$os-$arch"
base="https://github.com/$repo/releases/download/v$version"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fetch "$base/$asset" "$tmp/$asset" || build "could not download $asset for v$version"
fetch "$base/checksums.txt" "$tmp/checksums.txt" || build "could not download checksums for v$version"
want="$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
if command -v sha256sum >/dev/null 2>&1; then got="$(sha256sum "$tmp/$asset" | cut -d' ' -f1)"
else got="$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)"
fi
[ -n "$want" ] && [ "$want" = "$got" ] || build "checksum mismatch for $asset"
mv "$tmp/$asset" "$out"
chmod +x "$out"
echo "herdr-council: installed v$version ($os/$arch)"
