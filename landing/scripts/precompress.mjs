import { readdir, readFile, writeFile } from "node:fs/promises";
import { gzipSync, brotliCompressSync, constants } from "node:zlib";

// Keep originals for clients that do not advertise a supported encoding.
const root = new URL("../dist/landing-assets/", import.meta.url);
for (const entry of await readdir(root, { withFileTypes: true })) {
  if (!entry.isFile() || !/\.(js|css)$/.test(entry.name)) continue;
  const path = new URL(entry.name, root);
  const source = await readFile(path);
  const gzip = gzipSync(source, { level: 9 });
  const brotli = brotliCompressSync(source, {
    params: { [constants.BROTLI_PARAM_QUALITY]: 11 },
  });
  await Promise.all([
    writeFile(new URL(entry.name + ".gz", root), gzip),
    writeFile(new URL(entry.name + ".br", root), brotli),
  ]);
  console.log(
    `${entry.name}: ${source.length} → gzip ${gzip.length}, br ${brotli.length} bytes`,
  );
}
