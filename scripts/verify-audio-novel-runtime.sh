#!/bin/sh
set -eu

# Resolve Compose exactly as deployment does, then verify the rendered runtime contract.
config_file=$(mktemp)
trap 'rm -f "$config_file"' EXIT
docker compose -p linkscope config >"$config_file"

grep -F "AUDIO_NOVEL_DIR: /app/audio-novel" "$config_file" >/dev/null
grep -F "AUDIO_NOVEL_UPLOAD_DIR: /app/data/audio-novel-uploads" "$config_file" >/dev/null
grep -F "source: audio_novel_uploads" "$config_file" >/dev/null
grep -F "target: /app/data/audio-novel-uploads" "$config_file" >/dev/null

if grep -Eq '^[[:space:]]+NOVEL_(DIR|UPLOAD_DIR):' "$config_file"; then
  echo "retired novel environment variable remains in Compose" >&2
  exit 1
fi
if grep -Eq '^[[:space:]]+source: novel_uploads$' "$config_file"; then
  echo "retired novel upload volume remains in Compose" >&2
  exit 1
fi

echo "audio novel runtime contract verified"
