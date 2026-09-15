#!/bin/sh
# 使用同一组持久化目录续期；失败时停止，避免将失败报告为已完成。
set -eu
cd /opt/linkscope
/usr/local/bin/docker compose --env-file .env -f deploy/compose.production.yaml run --rm --no-deps certbot renew --non-interactive "$@"
# 平滑重载也适用于没有证书需要续期的情况，不会中断正在处理的请求。
/bin/sh /opt/linkscope/deploy/certbot-reload-nginx.sh
