#!/usr/bin/env bash
# 容器入口：处理 bind mount 数据目录的权限后降权运行。
#
# 为什么需要：容器内进程以 uid 5700（cabbage）运行，但 bind mount 的宿主机
# 目录属主通常是创建者（root 或普通用户），不一定是 5700。直接以 5700 启动会
# 因为无法写入 data/ 而启动失败。这里先以 root 修正属主，再降权执行。
#
# 命名卷场景下 Docker 会按镜像里 /app/data 的属主初始化，通常无需修正；
# 脚本里的属主检查会让这种情况直接跳过，不产生额外开销。
set -e

DATA_DIR="${CABBAGE_DATA_DIR:-/app/data}"
TARGET_UID=5700
TARGET_GID=5700

if [ "$(id -u)" = "0" ]; then
  mkdir -p "$DATA_DIR"
  if [ "$(stat -c '%u:%g' "$DATA_DIR")" != "$TARGET_UID:$TARGET_GID" ]; then
    echo "修正数据目录属主: $DATA_DIR -> $TARGET_UID:$TARGET_GID"
    chown -R "$TARGET_UID:$TARGET_GID" "$DATA_DIR" 2>/dev/null || \
      echo "警告: 无法修改 $DATA_DIR 属主；若写入失败，请在宿主机执行 chown -R $TARGET_UID:$TARGET_GID <数据目录>" >&2
  fi
  exec gosu "$TARGET_UID:$TARGET_GID" "$@"
fi

# 已是非 root 启动（例如 compose 里指定了 user:），直接执行。
exec "$@"
