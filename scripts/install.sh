#!/usr/bin/env bash
# 大白菜面板 Linux 一键安装脚本
#
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/bucai2417/cabbage-panel/main/scripts/install.sh | sudo bash
#   curl -fsSL .../install.sh | sudo bash -s -- --dir /opt/cabbage --port 5700
#
# 子命令（直接执行脚本时可用）：
#   install     安装或升级到最新版（默认）
#   upgrade     同上，保留数据
#   uninstall   卸载，默认保留数据目录
#   status      查看服务状态
#
# 常用参数：
#   --dir <路径>       安装目录，默认 /opt/cabbage
#   --port <端口>      监听端口，默认 5700
#   --version <版本>   安装指定版本，如 1.0.1；默认取最新 Release
#   --no-service       只装程序，不创建 systemd 服务
#   --purge            卸载时同时删除数据目录（危险）
set -euo pipefail

REPO="bucai2417/cabbage-panel"
INSTALL_DIR="/opt/cabbage"
PORT="5700"
VERSION=""
WITH_SERVICE=1
PURGE=0
ACTION="install"

SERVICE_NAME="cabbage"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
RUN_USER="cabbage"
DATA_DIR=""

# ---------- 输出 ----------
if [ -t 1 ]; then
  C_OK=$'\033[32m'; C_WARN=$'\033[33m'; C_ERR=$'\033[31m'; C_DIM=$'\033[2m'; C_END=$'\033[0m'
else
  C_OK=""; C_WARN=""; C_ERR=""; C_DIM=""; C_END=""
fi
info() { printf '%s==>%s %s\n' "$C_OK" "$C_END" "$*"; }
warn() { printf '%s警告:%s %s\n' "$C_WARN" "$C_END" "$*" >&2; }
fail() { printf '%s错误:%s %s\n' "$C_ERR" "$C_END" "$*" >&2; exit 1; }
dim()  { printf '%s%s%s\n' "$C_DIM" "$*" "$C_END"; }

# ---------- 参数 ----------
while [ $# -gt 0 ]; do
  case "$1" in
    install|upgrade|uninstall|status) ACTION="$1" ;;
    --dir) INSTALL_DIR="${2:?--dir 需要路径}"; shift ;;
    --port) PORT="${2:?--port 需要端口}"; shift ;;
    --version) VERSION="${2:?--version 需要版本号}"; shift ;;
    --no-service) WITH_SERVICE=0 ;;
    --purge) PURGE=1 ;;
    -h|--help)
      cat <<'USAGE'
大白菜面板 Linux 一键安装脚本

用法：
  curl -fsSL https://raw.githubusercontent.com/bucai2417/cabbage-panel/main/scripts/install.sh | sudo bash
  curl -fsSL .../install.sh | sudo bash -s -- --dir /opt/cabbage --port 5700

子命令（下载后直接执行时可用）：
  install     安装或升级到最新版（默认）
  upgrade     同上，保留数据
  uninstall   卸载，默认保留数据目录
  status      查看服务状态

常用参数：
  --dir <路径>       安装目录，默认 /opt/cabbage
  --port <端口>      监听端口，默认 5700
  --version <版本>   安装指定版本，如 1.0.1；默认取最新 Release
  --no-service       只装程序，不创建 systemd 服务
  --purge            卸载时同时删除数据目录（危险）
USAGE
      exit 0 ;;
    *) fail "未知参数: $1（用 --help 查看用法）" ;;
  esac
  shift
done

DATA_DIR="$INSTALL_DIR/data"

# ---------- 环境检查 ----------
need_root() {
  [ "$(id -u)" = "0" ] || fail "需要 root 权限，请用 sudo 执行"
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "缺少命令: $1"
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) fail "不支持的架构: $(uname -m)（仅支持 amd64 / arm64）" ;;
  esac
}

detect_os() {
  [ "$(uname -s)" = "Linux" ] || fail "本脚本仅支持 Linux；macOS 请参考 docs/BUILD.md 手动部署"
}

# ---------- 版本解析 ----------
latest_version() {
  # 优先用 GitHub API；被限流时回退到跟随 releases/latest 的重定向。
  local tag
  tag="$(curl -fsSL --max-time 20 "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
        | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\{0,1\}\([^"]*\)".*/\1/p' | head -1)"
  if [ -z "$tag" ]; then
    tag="$(curl -fsSI --max-time 20 "https://github.com/$REPO/releases/latest" 2>/dev/null \
          | sed -n 's|^[Ll]ocation:.*/tag/v\{0,1\}\([^[:space:]]*\).*|\1|p' | tr -d '\r' | head -1)"
  fi
  [ -n "$tag" ] || return 1
  echo "$tag"
}

# ---------- 下载 ----------
download() {
  local url="$1" out="$2"
  info "下载 $(basename "$url")"
  if ! curl -fL --retry 3 --retry-delay 2 --connect-timeout 15 -o "$out" "$url"; then
    return 1
  fi
}

verify_checksum() {
  local file="$1" sums="$2" name
  name="$(basename "$file")"
  [ -s "$sums" ] || { warn "未找到 SHA256SUMS，跳过校验"; return 0; }
  local expected
  expected="$(sed -n "s/^\([0-9a-f]\{64\}\)[[:space:]]\+\*\?${name}$/\1/p" "$sums" | head -1)"
  [ -n "$expected" ] || { warn "$name 不在 SHA256SUMS 中，跳过校验"; return 0; }
  local actual
  actual="$(sha256sum "$file" | cut -d' ' -f1)"
  [ "$expected" = "$actual" ] || fail "校验失败：$name 的 SHA-256 不匹配"
  info "SHA-256 校验通过"
}

# ---------- 动作 ----------
do_install() {
  need_root; detect_os; need_cmd curl; need_cmd tar
  local arch; arch="$(detect_arch)"

  if [ -z "$VERSION" ]; then
    info "查询最新版本"
    VERSION="$(latest_version)" || fail "无法获取最新版本，请用 --version 指定"
  fi
  VERSION="${VERSION#v}"

  local base="https://github.com/$REPO/releases/download/v$VERSION"
  local bin="cabbage-linux-$arch"
  local tmp; tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  download "$base/$bin" "$tmp/$bin" || fail "下载失败，请确认 v$VERSION 存在该平台构建"
  download "$base/SHA256SUMS" "$tmp/SHA256SUMS" || warn "SHA256SUMS 下载失败，跳过校验"
  verify_checksum "$tmp/$bin" "$tmp/SHA256SUMS"

  # 停旧服务，避免替换正在运行的程序。
  if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    info "停止运行中的服务"
    systemctl stop "$SERVICE_NAME"
  fi

  info "安装到 $INSTALL_DIR"
  mkdir -p "$INSTALL_DIR" "$DATA_DIR"

  # 创建运行用户（仅 systemd 模式需要）。
  if [ "$WITH_SERVICE" = "1" ] && ! id -u "$RUN_USER" >/dev/null 2>&1; then
    info "创建系统用户 $RUN_USER"
    useradd --system --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin "$RUN_USER" </dev/null 2>/dev/null \
      || useradd --system --home-dir "$INSTALL_DIR" --shell /sbin/nologin "$RUN_USER" </dev/null
  fi

  # 原子替换二进制：先写临时文件再 rename，避免替换过程中损坏。
  install -m 0755 "$tmp/$bin" "$INSTALL_DIR/cabbage.new"
  mv -f "$INSTALL_DIR/cabbage.new" "$INSTALL_DIR/cabbage"

  if [ "$WITH_SERVICE" = "1" ]; then
    info "配置 systemd 服务"
    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Cabbage Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$RUN_USER
Group=$RUN_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/cabbage --data $DATA_DIR --addr :$PORT
# 在线更新会就地替换程序并请求退出，靠 Restart=always 重新拉起新版本。
Restart=always
RestartSec=3
TimeoutStopSec=30
KillMode=control-group
NoNewPrivileges=true
PrivateTmp=true
UMask=0077
Environment=TZ=Asia/Shanghai
Environment=PYTHONUTF8=1

[Install]
WantedBy=multi-user.target
EOF
    chown -R "$RUN_USER:$RUN_USER" "$INSTALL_DIR"
    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME" >/dev/null 2>&1 || true
    systemctl restart "$SERVICE_NAME"
  else
    chown -R "$RUN_USER:$RUN_USER" "$INSTALL_DIR" 2>/dev/null || true
  fi

  local installed
  installed="$("$INSTALL_DIR/cabbage" --version 2>/dev/null | head -1)"
  info "安装完成：$installed"

  echo
  info "访问 http://<服务器IP>:$PORT"
  if [ "$WITH_SERVICE" = "1" ]; then
    echo
    dim "首次启动会生成设置密钥，从日志中获取："
    dim "  journalctl -u $SERVICE_NAME -n 20 --no-pager | grep 首次设置密钥"
    echo
    dim "常用命令："
    dim "  systemctl status $SERVICE_NAME     # 查看状态"
    dim "  systemctl restart $SERVICE_NAME    # 重启"
    dim "  journalctl -u $SERVICE_NAME -f     # 跟踪日志"
  fi
}

do_status() {
  need_root
  if ! systemctl list-unit-files "$SERVICE_NAME.service" >/dev/null 2>&1; then
    fail "未找到 $SERVICE_NAME 服务，可能尚未安装"
  fi
  systemctl status "$SERVICE_NAME" --no-pager || true
  echo
  if [ -x "$INSTALL_DIR/cabbage" ]; then
    dim "已安装版本: $("$INSTALL_DIR/cabbage" --version 2>/dev/null | head -1)"
    dim "安装目录:   $INSTALL_DIR"
    dim "数据目录:   $DATA_DIR"
  fi
}

do_uninstall() {
  need_root
  if systemctl list-unit-files "$SERVICE_NAME.service" >/dev/null 2>&1; then
    info "停止并禁用服务"
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
  fi
  rm -f "$SERVICE_FILE"
  systemctl daemon-reload 2>/dev/null || true

  info "删除程序文件"
  rm -f "$INSTALL_DIR/cabbage"

  if [ "$PURGE" = "1" ]; then
    warn "删除数据目录 $DATA_DIR（含数据库、脚本、备份）"
    rm -rf "$DATA_DIR"
  else
    dim "数据目录已保留：$DATA_DIR"
    dim "如需彻底删除：rm -rf $DATA_DIR"
  fi

  if id -u "$RUN_USER" >/dev/null 2>&1; then
    info "删除系统用户 $RUN_USER"
    userdel "$RUN_USER" 2>/dev/null || true
  fi
  info "卸载完成"
}

case "$ACTION" in
  install|upgrade) do_install ;;
  status) do_status ;;
  uninstall) do_uninstall ;;
esac
