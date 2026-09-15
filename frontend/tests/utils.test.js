import test from "node:test";
import assert from "node:assert/strict";
import { buildQuery, unitCost, validateLink } from "../src/utils.js";
test("filters omit empty values and preserve literal ad IDs", () =>
  assert.equal(
    buildQuery({ start: "2026-09-01", ad_id: "a&b", link_id: "" }),
    "start=2026-09-01&ad_id=a%26b",
  ));
test("cost has no denominator fabrication", () => {
  assert.equal(unitCost(12, 0), "—");
  assert.equal(unitCost(12, 3), "4.00");
});
test("WhatsApp destination rejects arbitrary URLs", () => {
  assert.ok(validateLink({ name: "Test", target_url: "https://evil.test/" }));
  assert.equal(
    validateLink({ name: "Test", target_url: "https://wa.me/13365661092" }),
    "",
  );
});
import { fillTrend } from "../src/utils.js";
test("trend fills empty calendar days without altering observed values", () =>
  assert.deepEqual(
    fillTrend([{ date: "2026-09-02", total: 4, filtered: 3, unique: 2 }], {
      start: "2026-09-01",
      end: "2026-09-03",
      tz: "Asia/Shanghai",
    }).map((x) => x.total),
    [0, 4, 0],
  ));
test("hourly trend excludes nonexistent spring DST hour", () => {
  const rows = fillTrend([], {
    start: "2026-03-08",
    end: "2026-03-08",
    tz: "America/New_York",
  });
  assert.equal(rows.length, 23);
  assert.ok(!rows.some((x) => x.date.endsWith("02:00")));
});
