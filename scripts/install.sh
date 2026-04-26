#!/usr/bin/env sh

set -eu

REPO="${REPO:-ruoruoji/clip2agent}"
VERSION="${VERSION:-latest}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
INSTALL_MACOS_HELPERS="${INSTALL_MACOS_HELPERS:-auto}"

usage() {
  cat <<'EOF'
Install clip2agent from GitHub Releases.

Usage:
  sh install.sh [--version v0.1.0|latest] [--bin-dir /path] [--with-macos-helpers] [--without-macos-helpers]

Environment variables:
  REPO                  GitHub repo, default: ruoruoji/clip2agent
  VERSION               Release version, default: latest
  BIN_DIR               Install directory, default: ~/.local/bin
  INSTALL_MACOS_HELPERS auto|true|false, default: auto
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      VERSION="$2"
      shift 2
      ;;
    --bin-dir)
      BIN_DIR="$2"
      shift 2
      ;;
    --with-macos-helpers)
      INSTALL_MACOS_HELPERS="true"
      shift
      ;;
    --without-macos-helpers)
      INSTALL_MACOS_HELPERS="false"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd tar
need_cmd mktemp

if command -v shasum >/dev/null 2>&1; then
  checksum_cmd="shasum -a 256"
elif command -v sha256sum >/dev/null 2>&1; then
  checksum_cmd="sha256sum"
else
  echo "missing checksum command: need shasum or sha256sum" >&2
  exit 1
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$os" in
  darwin) goos="darwin" ;;
  linux) goos="linux" ;;
  *)
    echo "unsupported OS: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if [ "$goos" = "darwin" ] && [ "$INSTALL_MACOS_HELPERS" = "auto" ]; then
  INSTALL_MACOS_HELPERS="true"
fi

if [ "$goos" != "darwin" ] && [ "$INSTALL_MACOS_HELPERS" = "auto" ]; then
  INSTALL_MACOS_HELPERS="false"
fi

version_path() {
  if [ "$VERSION" = "latest" ]; then
    printf 'latest/download'
  else
    printf 'download/%s' "$VERSION"
  fi
}

version_number() {
  if [ "$VERSION" = "latest" ]; then
    echo "latest"
  else
    echo "$VERSION" | sed 's/^v//'
  fi
}

download() {
  url="$1"
  dest="$2"
  echo "==> Downloading $(basename "$dest")"
  http_code=$(curl -sSL -w '%{http_code}' -o "$dest" "$url" || true)
  if [ -z "$http_code" ] || [ "$http_code" != "200" ]; then
    rm -f "$dest"
    echo "download failed (${http_code:-unknown}): $url" >&2
    return 1
  fi
}

verify_asset() {
  checksums_file="$1"
  asset_name="$2"
  asset_path="$3"
  expected=$(grep "  ${asset_name}$" "$checksums_file" | awk '{print $1}')
  if [ -z "$expected" ]; then
    echo "checksum not found for ${asset_name}" >&2
    exit 1
  fi
  actual=$($checksum_cmd "$asset_path" | awk '{print $1}')
  if [ "$expected" != "$actual" ]; then
    echo "checksum mismatch for ${asset_name}" >&2
    exit 1
  fi
}

extract_archive() {
  archive="$1"
  dest="$2"
  case "$archive" in
    *.zip)
      need_cmd unzip
      unzip -oq "$archive" -d "$dest"
      ;;
    *.tar.gz)
      tar -xzf "$archive" -C "$dest"
      ;;
    *)
      echo "unsupported archive: $archive" >&2
      exit 1
      ;;
  esac
}

install_binary() {
  src="$1"
  dest="$2"
  chmod +x "$src"
  cp "$src" "$dest"
}

ver=$(version_number)
asset_ext="tar.gz"
if [ "$goos" = "windows" ]; then
  asset_ext="zip"
fi
cli_asset="clip2agent_${ver}_${goos}_${goarch}.${asset_ext}"
helpers_asset="clip2agent_${ver}_${goos}_${goarch}_helpers.tar.gz"
checksums_asset="checksums.txt"
base_url="https://github.com/${REPO}/releases/$(version_path)"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM

mkdir -p "$BIN_DIR"

if ! download "$base_url/$checksums_asset" "$tmpdir/$checksums_asset"; then
  echo "==> GitHub Releases assets not found for ${REPO} (${VERSION}); falling back to 'go install'" >&2
  need_cmd go

  pkg="github.com/${REPO}/cmd/clip2agent"
  if [ "$VERSION" = "latest" ]; then
    goversion="@latest"
  else
    goversion="@${VERSION}"
  fi

  GOBIN="$BIN_DIR" go install "${pkg}${goversion}"
  echo "==> Installed to $BIN_DIR"
  echo "==> clip2agent version source: go install ${goversion}"

  if [ "$goos" = "darwin" ] && [ "$INSTALL_MACOS_HELPERS" = "true" ]; then
    echo "==> Note: macOS helpers are not installed in fallback mode; build from source if needed." >&2
  fi

  echo "==> Done"
  exit 0
fi

download "$base_url/$cli_asset" "$tmpdir/$cli_asset"
verify_asset "$tmpdir/$checksums_asset" "$cli_asset" "$tmpdir/$cli_asset"

mkdir -p "$tmpdir/cli"
extract_archive "$tmpdir/$cli_asset" "$tmpdir/cli"
install_binary "$tmpdir/cli/clip2agent" "$BIN_DIR/clip2agent"

if [ "$INSTALL_MACOS_HELPERS" = "true" ]; then
  download "$base_url/$helpers_asset" "$tmpdir/$helpers_asset"
  verify_asset "$tmpdir/$checksums_asset" "$helpers_asset" "$tmpdir/$helpers_asset"
  mkdir -p "$tmpdir/helpers"
  extract_archive "$tmpdir/$helpers_asset" "$tmpdir/helpers"
  install_binary "$tmpdir/helpers/clip2agent-macos" "$BIN_DIR/clip2agent-macos"
  install_binary "$tmpdir/helpers/clip2agent-hotkey" "$BIN_DIR/clip2agent-hotkey"
fi

echo "==> Installed to $BIN_DIR"
echo "==> clip2agent version source: $VERSION"

case ":$PATH:" in
  *":$BIN_DIR:"*)
    ;;
  *)
    echo "==> Add to PATH if needed: export PATH=\"$BIN_DIR:\$PATH\""
    ;;
esac

if [ "$goos" = "darwin" ] && [ "$INSTALL_MACOS_HELPERS" = "true" ]; then
  echo "==> Next step: run 'clip2agent setup' or 'clip2agent hotkey status'"
fi

echo "==> Done"
