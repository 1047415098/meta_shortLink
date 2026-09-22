import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

test("admin navigation separates each frontend, Meta and system into submenus", async () => {
  // 回归保护：三个前端项目必须保持独立分组，不能再次合并成一个内容菜单。
  const source = await readFile(
    new URL("../src/layouts/AdminLayout.vue", import.meta.url),
    "utf8",
  );
  assert.match(source, /<el-sub-menu/);
  assert.match(source, /label: "短链接项目"[\s\S]*name: "links"/);
  assert.match(source, /label: "语音小说项目"[\s\S]*name: "audio-novels"/);
  assert.match(source, /label: "免费小说项目"[\s\S]*name: "novels"/);
  assert.match(source, /label: "Meta 管理"[\s\S]*name: "meta-events"/);
  assert.match(source, /label: "系统"[\s\S]*name: "settings"/);
  assert.match(source, /:default-openeds="openGroups"/);
});
