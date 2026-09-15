import test from "node:test";
import assert from "node:assert/strict";
import { safeReturnPath } from "../src/utils/routes.js";
test("login returns to an admin page including its filters", () => {
  assert.equal(
    safeReturnPath("/admin/visits?link_id=7"),
    "/admin/visits?link_id=7",
  );
});
test("login redirect rejects external locations and login loops", () => {
  for (const value of [
    "https://example.com",
    "//example.com",
    "/admin/login",
    "/admin/\\evil",
    null,
    ["/admin/links"],
    "/admin/links\n",
  ])
    assert.equal(safeReturnPath(value), "/admin/overview");
});
