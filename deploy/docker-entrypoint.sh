#!/usr/bin/env bash
# 容器入口：确保数据目录可写后，以非 root 身份运行程序。
#
# 背景：容器内程序默认以 uid 5700 运行，但 bind mount 的宿主机目录属主往往
# 不是 5700。群晖等启用 Windows ACL 的共享文件夹更特殊：chown 会返回成功，
# 但 ACL 规则不认 POSIX 属主，目标 uid 依然写不进去。因此这里不只看 chown
# 结果，而是真实探测写入，探测失败时回退到 root，保证服务能起来。
#
# 可用环境变量：
#   CABBAGE_DATA_DIR  数据目录，默认 /app/data
#   PUID / PGID       运行身份，默认 5700:5700；设为宿主机目录属主可避免回退
set -e

DATA_DIR="${CABBAGE_DATA_DIR:-/app/data}"
RUN_UID="${PUID:-5700}"
RUN_GID="${PGID:-5700}"

# 非 root 启动（例如 compose 指定了 user:），无法 chown，直接执行。
if [ "$(id -u)" != "0" ]; then
  exec "$@"
fi

mkdir -p "$DATA_DIR"

if [ "$(stat -c '%u:%g' "$DATA_DIR")" != "$RUN_UID:$RUN_GID" ]; then
  echo "修正数据目录属主: $DATA_DIR -> $RUN_UID:$RUN_GID"
  chown -R "$RUN_UID:$RUN_GID" "$DATA_DIR" 2>/dev/null || true
fi

# 真实探测：以目标身份尝试创建文件。ACL 环境下 chown 成功不代表能写。
if TEST_DIR="$DATA_DIR" gosu "$RUN_UID:$RUN_GID" \
     sh -c 'touch "$TEST_DIR/.write-test" && rm -f "$TEST_DIR/.write-test"' 2>/dev/null; then
  exec gosu "$RUN_UID:$RUN_GID" "$@"
fi

echo "" >&2
echo "警告: uid $RUN_UID 无法写入 $DATA_DIR，本次以 root 运行。" >&2
echo "常见原因：群晖等启用 Windows ACL 的共享文件夹、NFS/SMB 挂载、目录属主不符。" >&2
echo "要恢复非 root 运行，任选其一：" >&2
echo "  1. 在宿主机查看数据目录属主：ls -nd <宿主机数据目录>" >&2
echo "     然后在 .env 设置 PUID/PGID 为该 uid/gid 并重建容器。" >&2
echo "  2. 在宿主机执行 chown -R $RUN_UID:$RUN_GID <宿主机数据目录>；" >&2
echo "     群晖还需在 File Station 的「权限」中给该 uid 授予读写。" >&2
echo "" >&2
exec "$@"
