#!/usr/bin/env bash
# 容器入口：自动适配数据目录权限后降权运行，无需任何手动配置。
#
# 原理：bind mount 保留宿主机的 uid/gid，容器内 stat /app/data 得到的就是宿主机
# 上该目录的真实属主。直接以这个属主运行，就能在群晖等启用 Windows ACL 的环境下
# 写入，不需要用户设置任何参数。
#
# 候选身份按优先级挑选，逐个真实探测写入（ACL 环境下 chown 成功不代表能写）：
#   1. PUID/PGID        用户显式指定
#   2. 数据目录当前属主   迁移过来的数据、用户自己建的目录
#   3. 父目录属主        docker 自动创建的 data 属主是 root，父目录才是真实用户
#   4. 5700:5700       镜像默认
#   5. root            兜底，保证服务一定能起来
#
# 只在候选身份与当前属主不符时 chown，且失败不中断——ACL 目录 chown 本来就可能
# 不生效，此时靠写入探测决定最终身份。
set -e

DATA_DIR="${CABBAGE_DATA_DIR:-/app/data}"
HOME_DIR=/home/cabbage
DEFAULT_UID=5700
DEFAULT_GID=5700

# 非 root 启动（compose 指定了 user:），无法 chown，直接执行。
if [ "$(id -u)" != "0" ]; then
  exec "$@"
fi

mkdir -p "$DATA_DIR"

# 在改动属主前记录原始属主，作为候选。
dir_owner="$(stat -c '%u %g' "$DATA_DIR" 2>/dev/null || echo "")"
parent_owner="$(stat -c '%u %g' "$(dirname "$DATA_DIR")" 2>/dev/null || echo "")"

candidates=""
add_candidate() {
  [ -z "$1" ] && return
  case " $candidates " in
    *" $1 "*) return ;;
  esac
  candidates="$candidates $1"
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

# 探测目标身份能否在数据目录里建文件。已有文件不参与判断——旧锁文件由
# 下面的清理步骤处理，否则 root 创建的 0600 锁会让所有非 root 候选都失败。
can_write() {
  TEST_DIR="$DATA_DIR" gosu "$1" sh -c '
    touch "$TEST_DIR/.write-test" || exit 1
    rm -f "$TEST_DIR/.write-test" || exit 1
  ' 2>/dev/null
}

CHOSEN=""
for candidate in $candidates; do
  # 先直接探测：属主已匹配时不需要任何改动，这是最常见的情况。
  if can_write "$candidate"; then
    CHOSEN="$candidate"
    break
  fi
  # 探测失败再尝试 chown 后重测；ACL 环境下可能仍失败，继续下一个候选。
  if [ "$(stat -c '%u:%g' "$DATA_DIR")" != "$candidate" ]; then
    echo "尝试身份 $candidate：修正属主后重测"
    chown -R "$candidate" "$DATA_DIR" 2>/dev/null || true
    if can_write "$candidate"; then
      CHOSEN="$candidate"
      break
    fi
  fi
  echo "跳过身份 $candidate（无法写入 $DATA_DIR）"
done

if [ -z "$CHOSEN" ]; then
  echo "警告: 所有候选身份都无法写入 $DATA_DIR，本次以 root 运行。" >&2
  echo "若希望以非 root 运行，在宿主机执行 chmod -R u+rwX <宿主机数据目录>；" >&2
  echo "群晖需在 File Station 的「权限」中给该目录授予读写。" >&2
  exec "$@"
fi

echo "运行身份: $CHOSEN"

# 选中身份后，确保已有文件都可读写。锁文件是 flock 用的空文件，不含数据，
# 权限不符直接删掉重建；数据库及其 WAL/SHM 不能删，改为尝试改属主，失败时
# 明确提示（否则启动后只会反复报 permission denied，很难定位）。
if [ "$CHOSEN" != "0:0" ]; then
  if [ -e "$DATA_DIR/cabbage.lock" ] && \
     ! TEST_DIR="$DATA_DIR" gosu "$CHOSEN" sh -c \
       'exec 3<>"$TEST_DIR/cabbage.lock"' 2>/dev/null; then
    echo "旧锁文件不可用，删除后重建: $DATA_DIR/cabbage.lock"
    rm -f "$DATA_DIR/cabbage.lock" 2>/dev/null || true
  fi
  for f in cabbage.db cabbage.db-wal cabbage.db-shm setup.key token.key; do
    path="$DATA_DIR/$f"
    [ -e "$path" ] || continue
    if ! TEST_FILE="$path" gosu "$CHOSEN" sh -c '[ -r "$TEST_FILE" ] && [ -w "$TEST_FILE" ]' 2>/dev/null; then
      echo "尝试修正 $f 属主 -> $CHOSEN"
      chown "$CHOSEN" "$path" 2>/dev/null || true
      if ! TEST_FILE="$path" gosu "$CHOSEN" sh -c '[ -r "$TEST_FILE" ] && [ -w "$TEST_FILE" ]' 2>/dev/null; then
        echo "警告: $f 仍无法被 $CHOSEN 读写，启动可能失败。" >&2
        echo "在宿主机执行: chown -R <数据目录> 并检查 ACL 权限" >&2
      fi
    fi
  done
fi

# 确保 HOME 可写，git 等工具需要。
mkdir -p "$HOME_DIR" && chown -R "$CHOSEN" "$HOME_DIR" 2>/dev/null || true

if [ "$CHOSEN" = "0:0" ]; then
  exec "$@"
fi
exec gosu "$CHOSEN" "$@"
