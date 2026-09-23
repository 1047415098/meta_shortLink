#!/bin/bash
# 仅用于首次部署：已有环境文件或数据卷时退出，防止覆盖凭证或已有业务数据。
set -euo pipefail
export LC_ALL=C
cd /opt/linkscope
test ! -e .env
if /usr/local/bin/docker volume inspect linkscope-lu81_postgres_data >/dev/null 2>&1; then
    echo 'Existing database volume detected; initialization stopped.'
    exit 1
fi
umask 077

# 凭证在服务器现场生成并仅写入权限为 600 的环境文件，不输出到日志。
database_password=$(openssl rand -hex 32)
application_secret=$(openssl rand -hex 32)
meta_key=$(openssl rand -base64 32)
cat > .env <<EOF
POSTGRES_PASSWORD=$database_password
ADMIN_USER=admin
ADMIN_PASSWORD=admin
APP_SECRET=$application_secret
META_ENCRYPTION_KEYS='{"primary":"$meta_key"}'
META_ENCRYPTION_KEY_ID=primary
# 在服务器现场填写 APIHZ 会员凭证；空值时仅禁用生成翻译，不影响英文小说。
APIHZ_TRANSLATION_ID=
APIHZ_TRANSLATION_KEY=
APIHZ_TRANSLATION_URL=https://cn.apihz.cn/api/zici/fanyiapihz.php
# 越南语使用 DeepL Free API；部署现场填写，不能写入仓库。
DEEPL_AUTH_KEY=
DEEPL_TRANSLATION_URL=https://api-free.deepl.com/v2/translate
COOKIE_MODE=all
TRUSTED_PROXIES=127.0.0.1
EOF
unset database_password application_secret meta_key
chmod 0600 .env

# 初始只启用 HTTP 配置；取得证书后再替换为 HTTPS 配置。
install -d -m 0755 deploy/runtime/nginx /var/www/letsencrypt/.well-known/acme-challenge
install -d -m 0700 /etc/letsencrypt /var/lib/letsencrypt /var/log/letsencrypt
install -m 0644 deploy/nginx.lu81.http.conf deploy/runtime/nginx/site.conf
/usr/local/bin/docker compose --env-file .env -f deploy/compose.production.yaml config --quiet
echo 'Fresh production environment initialized.'
