#!/usr/bin/env bash
# 容器入口：自动适配数据目录权限后降权运行。
#
# bind mount 保留宿主机的 uid/gid，容器内 stat /app/data 得到的就是宿主机上
# 该目录的真实属主。以它运行，就能在群晖等启用 Windows ACL 的环境下直接写入。
#
# 注意：不要用 set -e。这个脚本的所有探测失败都应当降级处理而不是直接退出，
# 否则容器会无日志地反复重启，很难定位。每一步失败都打印原因继续往下走。
set -u

log() { echo "[entrypoint] $*"; }
warn() { echo "[entrypoint] $*" >&2; }

log "启动，uid=$(id -u) 参数: $*"

DATA_DIR="${CABBAGE_DATA_DIR:-/app/data}"
HOME_DIR=/home/cabbage
DEFAULT_UID=5700
DEFAULT_GID=5700

if [ "$(id -u)" != "0" ]; then
  log "非 root 启动，直接执行"
  exec "$@"
fi

if ! command -v gosu >/dev/null 2>&1; then
  warn "gosu 不可用，以 root 运行"
  exec "$@"
fi

mkdir -p "$DATA_DIR" || warn "无法创建 $DATA_DIR"

dir_owner="$(stat -c '%u %g' "$DATA_DIR" 2>/dev/null || echo "")"
parent_owner="$(stat -c '%u %g' "$(dirname "$DATA_DIR")" 2>/dev/null || echo "")"
log "数据目录属主=[$dir_owner] 父目录属主=[$parent_owner]"

candidates=""
add_candidate() {
  [ -z "$1" ] && return 0
  case " $candidates " in
    *" $1 "*) return 0 ;;
  esac
  candidates="$candidates $1"
  return 0
}

if [ -n "${PUID:-}" ] && [ -n "${PGID:-}" ] && [ "$PUID:$PGID" != "0:0" ]; then
  add_candidate "$PUID:$PGID"
fi
case "$dir_owner" in
  ""|"0 0") ;;
  *) add_candidate "$(echo "$dir_owner" | tr ' ' ':')" ;;
esac
case "$parent_owner" in
  ""|"0 0") ;;
  *) add_candidate "$(echo "$parent_owner" | tr ' ' ':')" ;;
esac
add_candidate "$DEFAULT_UID:$DEFAULT_GID"
log "候选身份:$candidates"

can_write() {
  TEST_DIR="$DATA_DIR" gosu "$1" sh -c \
    'touch "$TEST_DIR/.write-test" && rm -f "$TEST_DIR/.write-test"' >/dev/null 2>&1
}

CHOSEN=""
for candidate in $candidates; do
  if can_write "$candidate"; then
    CHOSEN="$candidate"
    break
  fi
  if [ "$(stat -c '%u:%g' "$DATA_DIR" 2>/dev/null || echo "")" != "$candidate" ]; then
    log "尝试身份 $candidate：修正属主后重测"
    chown -R "$candidate" "$DATA_DIR" 2>/dev/null || true
    if can_write "$candidate"; then
      CHOSEN="$candidate"
      break
    fi
  fi
  log "跳过身份 $candidate（无法写入 $DATA_DIR）"
done

if [ -z "$CHOSEN" ]; then
  warn "所有候选身份都无法写入 $DATA_DIR，以 root 运行"
  exec "$@"
fi

log "运行身份: $CHOSEN"

# 锁文件是 flock 用的空文件，权限不符直接删掉重建；数据库及 WAL/SHM 不能删，
# 改为尝试改属主，仍失败则明确提示。
if [ "$CHOSEN" != "0:0" ]; then
  if [ -e "$DATA_DIR/cabbage.lock" ] && \
     ! TEST_DIR="$DATA_DIR" gosu "$CHOSEN" sh -c \
       'exec 3<>"$TEST_DIR/cabbage.lock"' >/dev/null 2>&1; then
    log "旧锁文件不可用，删除后重建"
    rm -f "$DATA_DIR/cabbage.lock" 2>/dev/null || true
  fi
  for f in cabbage.db cabbage.db-wal cabbage.db-shm setup.key token.key; do
    path="$DATA_DIR/$f"
    [ -e "$path" ] || continue
    if ! TEST_FILE="$path" gosu "$CHOSEN" sh -c \
         '[ -r "$TEST_FILE" ] && [ -w "$TEST_FILE" ]' >/dev/null 2>&1; then
      log "尝试修正 $f 属主 -> $CHOSEN"
      chown "$CHOSEN" "$path" 2>/dev/null || true
      if ! TEST_FILE="$path" gosu "$CHOSEN" sh -c \
           '[ -r "$TEST_FILE" ] && [ -w "$TEST_FILE" ]' >/dev/null 2>&1; then
        warn "警告: $f 仍无法被 $CHOSEN 读写"
      fi
    fi
  done
fi

mkdir -p "$HOME_DIR" 2>/dev/null && chown -R "$CHOSEN" "$HOME_DIR" 2>/dev/null || true

if [ "$CHOSEN" = "0:0" ]; then
  exec "$@"
fi
exec gosu "$CHOSEN" "$@"
