#!/usr/bin/env sh

set -eu

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
PURGE="${PURGE:-false}"
DRY_RUN="${DRY_RUN:-false}"
YES="${YES:-false}"

usage() {
  cat <<'EOF'
卸载 clip2agent。

用法：
  sh uninstall.sh [--bin-dir /path] [--dry-run] [--purge --yes]

环境变量：
  BIN_DIR   二进制卸载目录，默认：~/.local/bin
  DRY_RUN   true|false，默认：false
  PURGE     true|false，默认：false（危险：通过 CLI 删除 kept-dir）
  YES       true|false，默认：false（PURGE=true 时必填）

说明：
  - 推荐优先调用：$BIN_DIR/clip2agent uninstall --bin-dir $BIN_DIR
  - 本地开发重置建议先执行 dry-run，再执行: clip2agent uninstall --purge --yes
  - 如果 clip2agent 不存在，脚本只会回退到最小清理，不代表完整开发态 reset
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
      echo "未知参数: $1" >&2
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
  echo "==> 通过 $cli 执行卸载"
  # shellcheck disable=SC2086
  "$cli" $cli_args
  echo "==> 完成"
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
    echo "plan: remove $HOME/Library/Logs/clip2agent.log"
  fi
  if [ "$os" = "linux" ]; then
    echo "plan: remove $xdg_cfg/clip2agent/xbindkeys.conf"
    echo "plan: remove $xdg_cfg/autostart/clip2agent-xbindkeys.desktop"
  fi
  echo "next: 确认清理范围后，再执行彻底清理"
  echo "next: clip2agent uninstall --purge --yes"
  echo "==> 完成"
  exit 0
fi

echo "==> 未在 $BIN_DIR 找到 clip2agent；回退到最小清理" >&2

if [ "$os" = "darwin" ]; then
  uid=$(id -u 2>/dev/null || echo "")
  if [ -n "$uid" ]; then
    launchctl bootout "gui/${uid}/dev.clip2agent-hotkey" >/dev/null 2>&1 || true
  fi
  rm -f "$HOME/Library/LaunchAgents/dev.clip2agent-hotkey.plist" || true
  rm -f "$HOME/Library/Logs/clip2agent.log" || true
fi

if [ "$os" = "linux" ]; then
  xdg_cfg="${XDG_CONFIG_HOME:-$HOME/.config}"
  rm -f "$xdg_cfg/clip2agent/xbindkeys.conf" || true
  rm -f "$xdg_cfg/autostart/clip2agent-xbindkeys.desktop" || true
fi

xdg_cfg="${XDG_CONFIG_HOME:-$HOME/.config}"
rm -f "$xdg_cfg/clip2agent/hotkey.json" || true
rm -f "$BIN_DIR/clip2agent" "$BIN_DIR/clip2agent-macos" "$BIN_DIR/clip2agent-hotkey" || true

echo "==> 完成（最小清理；如需完整开发态重置，请恢复 CLI 后执行 clip2agent uninstall --purge --yes）"
