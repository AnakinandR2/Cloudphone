#!/usr/bin/env bash
# 打包脚本：分别构建 backend / www / admin / my 到 ops/dist/<名字> 下。
#   - backend：编译 Go 二进制，并把当前 backend/.env 一并拷过去
#   - www    ：nuxt build，产物 .output → ops/dist/www
#   - admin  ：vite build，产物 dist → ops/dist/admin
#   - my     ：vite build，产物 dist → ops/dist/my
#
# 用法：
#   ./ops/build.sh              # 全部构建
#   ./ops/build.sh backend my   # 只构建指定目标
set -euo pipefail

# 仓库根目录 = 本脚本所在 ops/ 的上一级
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# 产物输出目录（可用 OUT_DIR 覆盖）。各项目分别落到 $DIST/{backend,www,admin,my}。
DIST="${OUT_DIR:-/var/www/gloryphone}"

# 后端二进制名（可用 BIN_NAME 覆盖）
BIN_NAME="${BIN_NAME:-manager-backend}"

# 默认构建全部；也可传参指定子集。
TARGETS=("$@")
if [ ${#TARGETS[@]} -eq 0 ]; then
  TARGETS=(backend www admin my)
fi

log()  { printf '\033[1;36m[build]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[warn]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

# 前端：node_modules 不存在才安装，避免每次都装。
ensure_deps() {
  local dir="$1"
  if [ ! -d "$dir/node_modules" ]; then
    log "$dir: 安装依赖（首次）"
    (cd "$dir" && pnpm install --frozen-lockfile)
  fi
}

# 把 src 目录内容原样拷到 dst（先清空 dst）。
sync_dir() {
  local src="$1" dst="$2"
  rm -rf "$dst"
  mkdir -p "$dst"
  cp -a "$src/." "$dst/"
}

build_backend() {
  log "backend: go build → $DIST/backend/$BIN_NAME"
  command -v go >/dev/null || die "未找到 go"
  # 不清理 backend 目录：里面可能有 sqlite 数据库等运行时数据，只覆盖二进制。
  mkdir -p "$DIST/backend"
  # 不强制 CGO_ENABLED=0：依赖 mattn/go-sqlite3（需 CGO），关掉会丢 sqlite 支持。
  (cd "$ROOT/backend" && go build -trimpath -o "$DIST/backend/$BIN_NAME" .)
  log "backend: 已更新二进制"
  # .env：仅当目标不存在时拷贝（避免覆盖线上已调好的配置）；FORCE_ENV=1 强制用源 .env 覆盖。
  if [ -f "$ROOT/backend/.env" ]; then
    if [ ! -f "$DIST/backend/.env" ] || [ "${FORCE_ENV:-0}" = "1" ]; then
      cp -a "$ROOT/backend/.env" "$DIST/backend/.env"
      log "backend: 已写入 .env"
    else
      log "backend: 保留已存在的 .env（如需用源 .env 覆盖：FORCE_ENV=1）"
    fi
  else
    warn "backend/.env 不存在，跳过（可参考 backend/.env.example）"
  fi
}

build_www() {
  log "www: nuxt build → $DIST/www"
  ensure_deps "$ROOT/www"
  (cd "$ROOT/www" && pnpm build)
  [ -d "$ROOT/www/.output" ] || die "www 构建产物 .output 不存在"
  sync_dir "$ROOT/www/.output" "$DIST/www"
}

build_vite() {
  local name="$1"
  log "$name: vite build → $DIST/$name"
  ensure_deps "$ROOT/$name"
  (cd "$ROOT/$name" && pnpm build)
  [ -d "$ROOT/$name/dist" ] || die "$name 构建产物 dist 不存在"
  sync_dir "$ROOT/$name/dist" "$DIST/$name"
}

mkdir -p "$DIST"
for t in "${TARGETS[@]}"; do
  case "$t" in
    backend) build_backend ;;
    www)     build_www ;;
    admin)   build_vite admin ;;
    my)      build_vite my ;;
    *)       die "未知目标：$t（可选 backend www admin my）" ;;
  esac
done

log "完成。产物在 $DIST/"
ls -1 "$DIST"
