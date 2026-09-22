import { readdir, readFile, writeFile } from "node:fs/promises";
import { gzipSync, brotliCompressSync, constants } from "node:zlib";

// 保留原文件，同时生成服务端可直接返回的 gzip 与 Brotli 版本。
const root = new URL("../dist/novel-assets/", import.meta.url);
for (const entry of await readdir(root, { withFileTypes: true })) {
  if (!entry.isFile() || !/\.(js|css)$/.test(entry.name)) continue;
  const path = new URL(entry.name, root), source = await readFile(path);
  await Promise.all([
    writeFile(new URL(entry.name + ".gz", root), gzipSync(source, { level: 9 })),
    writeFile(new URL(entry.name + ".br", root), brotliCompressSync(source, { params: { [constants.BROTLI_PARAM_QUALITY]: 11 } }))
  ]);
}
