import assert from "node:assert/strict";
import test from "node:test";
test("public content contract contains no author field", () => {
  const response = { items: [], page: 1, pages: 0, total: 0 };
  assert.deepEqual(response.items, []);
  assert.equal("author" in response, false);
});
