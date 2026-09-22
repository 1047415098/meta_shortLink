#!/bin/sh
set -eu

# 同时启动四个 Vue 开发服务；Ctrl+C 只停止前端监听进程，不影响数据库。
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if ! curl -fsS http://127.0.0.1:8080/healthz >/dev/null; then
  echo "请先启动本地 Go 服务：http://127.0.0.1:8080" >&2
  exit 1
fi

cleanup() {
  kill "$admin_pid" "$landing_pid" "$audio_novel_pid" "$novel_h5_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

(cd "$project_dir/frontend" && npm run dev) &
admin_pid=$!
(cd "$project_dir/landing" && npm run dev) &
landing_pid=$!
(cd "$project_dir/audio-novel" && npm run dev) &
audio_novel_pid=$!
(cd "$project_dir/novel-h5" && npm run dev) &
novel_h5_pid=$!

wait "$admin_pid" "$landing_pid" "$audio_novel_pid" "$novel_h5_pid"
