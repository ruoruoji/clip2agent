#!/usr/bin/env sh

set -eu

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
PURGE="${PURGE:-false}"
DRY_RUN="${DRY_RUN:-false}"
YES="${YES:-false}"

usage() {
  cat <<'EOF'
Uninstall clip2agent.

Usage:
  sh uninstall.sh [--bin-dir /path] [--dry-run] [--purge --yes]

Environment variables:
  BIN_DIR   Uninstall directory for binaries, default: ~/.local/bin
  DRY_RUN   true|false, default: false
  PURGE     true|false, default: false (danger: remove kept-dir via CLI)
  YES       true|false, default: false (required with PURGE)

Notes:
  - Prefer calling: $BIN_DIR/clip2agent uninstall --bin-dir $BIN_DIR
  - If clip2agent is missing, falls back to minimal removal only.
EOF
}

is_true() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|y|Y) return 0 ;;
    *) return 1 ;;
  esac
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --bin-dir)
      BIN_DIR="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    --purge)
      PURGE="true"
      shift
      ;;
    --yes)
      YES="true"
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

mkdir -p "$BIN_DIR"

if is_true "$PURGE" && ! is_true "$YES"; then
  echo "PURGE=true 需要同时 YES=true（危险操作确认）" >&2
  echo "示例：curl .../uninstall.sh | PURGE=true YES=true sh" >&2
  exit 1
fi

cli="$BIN_DIR/clip2agent"
cli_args="uninstall --bin-dir $BIN_DIR"
if is_true "$DRY_RUN"; then
  cli_args="$cli_args --dry-run"
fi
if is_true "$PURGE"; then
  cli_args="$cli_args --purge --yes"
fi

if [ -x "$cli" ]; then
  echo "==> Uninstalling via $cli"
  # shellcheck disable=SC2086
  "$cli" $cli_args
  echo "==> Done"
  exit 0
fi

if is_true "$PURGE"; then
  echo "PURGE=true 需要 clip2agent 可执行文件来安全删除 kept-dir（请先安装/确保 BIN_DIR 正确）" >&2
  exit 1
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')

if is_true "$DRY_RUN"; then
  echo "plan: remove $BIN_DIR/clip2agent"
  echo "plan: remove $BIN_DIR/clip2agent-macos"
  echo "plan: remove $BIN_DIR/clip2agent-hotkey"
  xdg_cfg="${XDG_CONFIG_HOME:-$HOME/.config}"
  echo "plan: remove $xdg_cfg/clip2agent/hotkey.json"
  if [ "$os" = "darwin" ]; then
    echo "plan: launchctl bootout gui/$(id -u)/dev.clip2agent-hotkey (best-effort)"
    echo "plan: remove $HOME/Library/LaunchAgents/dev.clip2agent-hotkey.plist"
    echo "plan: remove $HOME/Library/Logs/clip2agent-hotkey.log"
  fi
  if [ "$os" = "linux" ]; then
    echo "plan: remove $xdg_cfg/clip2agent/xbindkeys.conf"
    echo "plan: remove $xdg_cfg/autostart/clip2agent-xbindkeys.desktop"
  fi
  echo "==> Done"
  exit 0
fi

echo "==> clip2agent not found in $BIN_DIR; falling back to minimal removal" >&2

if [ "$os" = "darwin" ]; then
  uid=$(id -u 2>/dev/null || echo "")
  if [ -n "$uid" ]; then
    launchctl bootout "gui/${uid}/dev.clip2agent-hotkey" >/dev/null 2>&1 || true
  fi
  rm -f "$HOME/Library/LaunchAgents/dev.clip2agent-hotkey.plist" || true
  rm -f "$HOME/Library/Logs/clip2agent-hotkey.log" || true
fi

if [ "$os" = "linux" ]; then
  xdg_cfg="${XDG_CONFIG_HOME:-$HOME/.config}"
  rm -f "$xdg_cfg/clip2agent/xbindkeys.conf" || true
  rm -f "$xdg_cfg/autostart/clip2agent-xbindkeys.desktop" || true
fi

xdg_cfg="${XDG_CONFIG_HOME:-$HOME/.config}"
rm -f "$xdg_cfg/clip2agent/hotkey.json" || true
rm -f "$BIN_DIR/clip2agent" "$BIN_DIR/clip2agent-macos" "$BIN_DIR/clip2agent-hotkey" || true

echo "==> Done"
