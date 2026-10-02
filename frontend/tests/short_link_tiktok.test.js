import test from "node:test";
import assert from "node:assert/strict";

import { tiktokTemplateURL } from "../src/utils/tiktok.js";
import * as linkUtils from "../src/utils.js";

const { validateLink } = linkUtils;

test("short-link payload keeps exactly one advertising platform binding", () => {
  assert.equal(typeof linkUtils.shortLinkPayload, "function");
  assert.deepEqual(
    linkUtils.shortLinkPayload({
      name: " TikTok buyer ",
      target_url: "https://wa.me/13365661092",
      ad_platform: "tiktok",
      meta_connection_id: 2,
      meta_pixel_id: 3,
      tiktok_pixel_id: 9,
      attribution_mode: "dynamic",
    }),
    {
      name: "TikTok buyer",
      target_url: "https://wa.me/13365661092",
      ad_platform: "tiktok",
      meta_connection_id: null,
      meta_pixel_id: null,
      tiktok_pixel_id: 9,
      attribution_mode: "dynamic",
    },
  );
});

test("short-link validation accepts TikTok and rejects mixed or missing Pixels", () => {
  const base = {
    name: "TikTok buyer",
    target_url: "https://wa.me/13365661092",
    attribution_mode: "dynamic",
    ad_platform: "tiktok",
    tiktok_pixel_id: 9,
    meta_connection_id: null,
    meta_pixel_id: null,
  };
  assert.equal(validateLink(base), "");
  assert.equal(
    validateLink({ ...base, tiktok_pixel_id: null }),
    "请选择 TikTok Pixel",
  );
  assert.equal(
    validateLink({ ...base, meta_pixel_id: 3 }),
    "一个短链接只能绑定一个广告平台",
  );
});

test("TikTok short-link template uses the root short code and dynamic ad macros", () => {
  assert.equal(
    tiktokTemplateURL("https://example.com/", "buyer-a", "short-link"),
    "https://example.com/buyer-a?utm_source=tiktok&utm_medium=paid_social&campaign_id=__CAMPAIGN_ID__&adgroup_id=__AID__&creative_id=__CID__&ad_id_v2=__ADID_V2__&placement=__PLACEMENT__",
  );
});
