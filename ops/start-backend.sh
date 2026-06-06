#!/usr/bin/env bash
# 附带环境变量启动后端：加载 dist/backend/.env 到进程环境，再运行编译好的二进制。
# 先跑 ./ops/build.sh backend 生成产物。
#
# 用法：
#   ./ops/start-backend.sh                 # 用 dist/backend/.env 启动
#   SERVER_PORT=8080 ./ops/start-backend.sh   # 命令行附加/覆盖环境变量
#   ENV_FILE=/path/to/.env ./ops/start-backend.sh   # 指定其它 env 文件
set -euo pipefail

DIST_BACKEND="${BACKEND_DIST:-/var/www/gloryphone/backend}"
BIN_NAME="${BIN_NAME:-manager-backend}"
BIN="$DIST_BACKEND/$BIN_NAME"
ENV_FILE="${ENV_FILE:-$DIST_BACKEND/.env}"

log()  { printf '\033[1;36m[start-backend]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[warn]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

[ -x "$BIN" ] || die "未找到后端二进制：$BIN（先运行 ./ops/build.sh backend）"

# 解析 .env 并导出到环境。自己解析（不用 source）以兼容 Windows CRLF 行尾、
# 注释、成对引号；行为对齐后端 loadDotEnv：已存在的环境变量不覆盖（命令行预设优先）。
if [ -f "$ENV_FILE" ]; then
  log "加载环境变量：$ENV_FILE"
  while IFS= read -r line || [ -n "$line" ]; do
    line="${line%$'\r'}"                       # 去掉 CRLF 的 \r
    line="${line#"${line%%[![:space:]]*}"}"    # 去掉行首空白
    case "$line" in '' | '#'*) continue ;; esac
    [ "${line%%=*}" = "$line" ] && continue    # 无 '=' → 跳过
    key="${line%%=*}"
    val="${line#*=}"
    key="${key//[[:space:]]/}"                 # 键去空白
    val="${val#"${val%%[![:space:]]*}"}"       # 值去首尾空白
    val="${val%"${val##*[![:space:]]}"}"
    case "$val" in                             # 去成对引号
      \"*\") val="${val#\"}"; val="${val%\"}" ;;
      \'*\') val="${val#\'}"; val="${val%\'}" ;;
    esac
    printenv "$key" >/dev/null 2>&1 && continue # 已有同名环境变量 → 不覆盖
    export "$key=$val"
  done < "$ENV_FILE"
else
  warn "未找到 $ENV_FILE，使用二进制内置默认配置"
fi

# cd 到产物目录：后端的 loadDotEnv 与 sqlite 等相对路径都基于工作目录。
cd "$DIST_BACKEND"
log "启动后端，端口 ${SERVER_PORT:-9981}"
exec "./$BIN_NAME" "$@"
