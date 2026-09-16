import test from "node:test";
import assert from "node:assert/strict";
import {
  buildQuery,
  locationLabel,
  locationVisitTotal,
  unitCost,
  validateLink,
} from "../src/utils.js";
test("filters omit empty values and preserve literal ad IDs", () =>
  assert.equal(
    buildQuery({ start: "2026-09-01", ad_id: "a&b", link_id: "" }),
    "start=2026-09-01&ad_id=a%26b",
  ));
test("cost has no denominator fabrication", () => {
  assert.equal(unitCost(12, 0), "—");
  assert.equal(unitCost(12, 3), "4.00");
});
test("location summary and remainder partition visits without duplication", () => {
  const locations = [
    { visits: 2 },
    { visits: 1 },
    { visits: 1 },
    { visits: 1 },
    { visits: 1 },
    { visits: 1 },
    { visits: 1 },
    { visits: 1 },
  ];
  assert.equal(locationVisitTotal(locations), 9);
  assert.equal(locationVisitTotal(locations.slice(0, 3)), 4);
  assert.equal(locationVisitTotal(locations.slice(3)), 5);
});
test("WhatsApp destination rejects arbitrary URLs", () => {
  const valid = {
    name: "Test",
    target_url: "https://wa.me/13365661092",
    meta_pixel_id: 7,
    attribution_mode: "dynamic",
  };
  assert.ok(validateLink({ ...valid, target_url: "https://evil.test/" }));
  assert.equal(validateLink(valid), "");
});
test("short links require a Pixel and an explicit attribution mode", () => {
  const valid = {
    name: "Test",
    target_url: "https://wa.me/13365661092",
    meta_pixel_id: 7,
    attribution_mode: "dynamic",
  };
  // A link without a concrete CAPI destination cannot satisfy the reporting contract.
  assert.equal(
    validateLink({ ...valid, meta_pixel_id: null }),
    "请选择 Meta Pixel",
  );
  assert.equal(
    validateLink({ ...valid, attribution_mode: "" }),
    "请选择广告归因方式",
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
// Location labels preserve GeoIP values and make missing levels explicit to operators.
test("ad location label includes country, region and city fallbacks", () => {
  assert.equal(
    locationLabel({ country: "US", region: "Washington", city: "Seattle" }),
    "US / Washington / Seattle",
  );
  assert.equal(
    locationLabel({ country: "", region: "", city: "" }),
    "未知国家 / 未知州省 / 未知城市",
  );
  assert.equal(
    locationLabel({ country: "unknown", region: "unknown", city: "unknown" }),
    "未知国家 / 未知州省 / 未知城市",
  );
});
