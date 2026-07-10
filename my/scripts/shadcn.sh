#!/usr/bin/env bash
# shadcn-vue CLI 包装：注入代理开关与系统 CA，使其能在本机特殊网络下联网
# 用法: ./scripts/shadcn.sh add button dialog ...
set -e
export NODE_USE_ENV_PROXY=1
export NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt
exec pnpm dlx shadcn-vue@latest "$@"
