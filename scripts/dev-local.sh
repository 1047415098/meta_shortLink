#!/bin/sh
set -eu

# Start both Vue development servers while the single Go API keeps running on
# port 8080. One Ctrl+C stops both watchers without touching Docker or data.
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if ! curl -fsS http://127.0.0.1:8080/healthz >/dev/null; then
  echo "请先启动本地 Go 服务：http://127.0.0.1:8080" >&2
  exit 1
fi

cleanup() {
  kill "$admin_pid" "$landing_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

(cd "$project_dir/frontend" && npm run dev) &
admin_pid=$!
(cd "$project_dir/landing" && npm run dev) &
landing_pid=$!

wait "$admin_pid" "$landing_pid"
