#!/bin/bash
# 适用于已确认没有 Docker 的 x86_64 服务器；不覆盖已有运行环境。
set -euo pipefail
export LC_ALL=C
if command -v docker >/dev/null 2>&1; then
    echo 'Docker already exists; inspect the installation instead of replacing it.'
    exit 1
fi
test "$(uname -m)" = x86_64
install -d -m 0755 /usr/local/bin /usr/local/lib/docker/cli-plugins /etc/docker
install -d -m 0700 /opt/linkscope-bootstrap
cd /opt/linkscope-bootstrap

# 固定本次部署核实过的官方版本；CLI 插件额外比对官方 SHA-256 文件。
curl -fL --retry 2 --connect-timeout 15 --max-time 240 -o docker.tgz https://download.docker.com/linux/static/stable/x86_64/docker-29.8.0.tgz
tar -xzf docker.tgz
install -m 0755 docker/* /usr/local/bin/
curl -fL --retry 2 --connect-timeout 15 --max-time 240 -o docker-compose-linux-x86_64 https://github.com/docker/compose/releases/download/v5.5.1/docker-compose-linux-x86_64
curl -fL --retry 2 --connect-timeout 15 --max-time 60 -o docker-compose-linux-x86_64.sha256 https://github.com/docker/compose/releases/download/v5.5.1/docker-compose-linux-x86_64.sha256
sha256sum -c docker-compose-linux-x86_64.sha256
install -m 0755 docker-compose-linux-x86_64 /usr/local/lib/docker/cli-plugins/docker-compose
curl -fL --retry 2 --connect-timeout 15 --max-time 240 -o buildx-v0.37.1.linux-amd64 https://github.com/docker/buildx/releases/download/v0.37.1/buildx-v0.37.1.linux-amd64
curl -fL --retry 2 --connect-timeout 15 --max-time 60 -o checksums.txt https://github.com/docker/buildx/releases/download/v0.37.1/checksums.txt
# SHA-256 清单可能用空格或星号表示文本/二进制，两种格式都交给 sha256sum 验证。
grep '[ *]buildx-v0.37.1.linux-amd64$' checksums.txt | sha256sum -c -
install -m 0755 buildx-v0.37.1.linux-amd64 /usr/local/lib/docker/cli-plugins/docker-buildx

# 只在本机 Unix socket 提供管理接口；限制容器日志占用磁盘。
groupadd -f docker
cat > /etc/docker/daemon.json <<'JSON'
{
  "log-driver": "json-file",
  "log-opts": {"max-size": "20m", "max-file": "3"},
  "live-restore": true
}
JSON
install -m 0644 /opt/linkscope-bootstrap/docker.service /etc/systemd/system/docker.service
systemctl daemon-reload
systemctl enable --now docker
/usr/local/bin/docker version
/usr/local/bin/docker compose version
sha256sum docker.tgz
