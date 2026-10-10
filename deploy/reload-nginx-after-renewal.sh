#!/bin/sh

# Certbot 续期成功后先校验宝塔 Nginx 配置，通过后再平滑重载新证书。
set -eu

/www/server/nginx/sbin/nginx -t
/www/server/nginx/sbin/nginx -s reload
