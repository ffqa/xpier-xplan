#!/bin/sh
# Install the latest xplan release for this machine.
#
#   curl -fsSL https://raw.githubusercontent.com/ffqa/xpier-xplan/main/scripts/install.sh | sh
#
# XPLAN_INSTALL_DIR overrides the target directory (default ~/.local/bin).
# XPLAN_VERSION installs a specific tag, e.g. v0.5.0.
set -eu

repo="ffqa/xpier-xplan"
install_dir="${XPLAN_INSTALL_DIR:-$HOME/.local/bin}"

fail() {
  echo "xplan install: $*" >&2
  exit 1
}

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "unsupported system $(uname -s); on Windows download the zip from https://github.com/$repo/releases" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported CPU $(uname -m)" ;;
esac

if [ -n "${XPLAN_VERSION:-}" ]; then
  base="https://github.com/$repo/releases/download/$XPLAN_VERSION"
else
  base="https://github.com/$repo/releases/latest/download"
fi
asset="xplan_${os}_${arch}.tar.gz"

command -v curl >/dev/null 2>&1 || fail "curl is required"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Downloading $asset ..."
curl -fsSL "$base/$asset" -o "$work/$asset" || fail "download failed: $base/$asset"
curl -fsSL "$base/checksums.txt" -o "$work/checksums.txt" || fail "download failed: $base/checksums.txt"

expected="$(grep " $asset\$" "$work/checksums.txt" | cut -d' ' -f1)"
[ -n "$expected" ] || fail "$asset is not listed in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$work/$asset" | cut -d' ' -f1)"
else
  actual="$(shasum -a 256 "$work/$asset" | cut -d' ' -f1)"
fi
[ "$expected" = "$actual" ] || fail "checksum mismatch for $asset"

tar -xzf "$work/$asset" -C "$work"
mkdir -p "$install_dir"
mv "$work/xplan_${os}_${arch}/xplan" "$install_dir/xplan"
chmod +x "$install_dir/xplan"

echo "Installed $("$install_dir/xplan" version) to $install_dir/xplan"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "Add $install_dir to your PATH to run xplan from anywhere." ;;
esac
echo
echo "Next:"
echo "  xplan service install    # keep xplan running in the background (scheduled reviews, notifications, MCP)"
echo "  open http://127.0.0.1:7788"
