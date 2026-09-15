#!/bin/sh
# 宿主机续期任务调用：先验证容器内站点配置，再平滑加载新证书。
set -eu
cd /opt/linkscope
/usr/local/bin/docker compose --env-file .env -f deploy/compose.production.yaml exec -T nginx nginx -t
/usr/local/bin/docker compose --env-file .env -f deploy/compose.production.yaml exec -T nginx nginx -s reload
