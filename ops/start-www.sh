#!/usr/bin/env bash
# 启动 www（Nuxt 产物，Nitro Node 服务）。先跑 ./ops/build.sh www 生成产物。
#
# 用法：
#   ./ops/start-www.sh                 # 默认 0.0.0.0:3000
#   PORT=8080 HOST=127.0.0.1 ./ops/start-www.sh
#   NUXT_BACKEND_BASE_URL=http://127.0.0.1:9981/api/v1 ./ops/start-www.sh  # 覆盖服务端访问后端地址
set -euo pipefail

WWW_DIST="${WWW_DIST:-/var/www/gloryphone/www}"
ENTRY="$WWW_DIST/server/index.mjs"

log()  { printf '\033[1;36m[start-www]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

command -v node >/dev/null || die "未找到 node"
[ -f "$ENTRY" ] || die "未找到 www 产物：$ENTRY（先运行 ./ops/build.sh www）"

# Nitro 读 HOST / PORT；NUXT_BACKEND_BASE_URL 映射 runtimeConfig.backendBaseUrl。
export HOST="${HOST:-0.0.0.0}"
export PORT="${PORT:-3000}"

log "启动 www：http://$HOST:$PORT"
exec node "$ENTRY"
