import test from "node:test";
import assert from "node:assert/strict";
import { buildMetaAttributionHeaders } from "../src/lib/attribution.js";

// Breaking the field whitelist, canonical fbclid choice, or UTF-8 encoding must fail this contract test.
test("Meta attribution headers contain only resolved approved URL fields", () => {
  const headers = buildMetaAttributionHeaders(
    "?fbclid=canonical-click&fbcli=alias-click&utm_source=FACEBOOK&utm_campaign=%E5%B9%BF%E5%91%8A%E7%B3%BB%E5%88%97&utm_content=1201&adset_name=Prospects&adset_id=2202&ad_id=3303&ad_name=%E5%B9%BF%E5%91%8A-A&placement=instagram_story&site_source_name=ig&token=secret&campaign_id=%7B%7Bcampaign.id%7D%7D",
  );
  assert.deepEqual(headers, {
    "X-Meta-Fbclid": "canonical-click",
    "X-Meta-Utm-Source": "FACEBOOK",
    "X-Meta-Utm-Campaign": "%E5%B9%BF%E5%91%8A%E7%B3%BB%E5%88%97",
    "X-Meta-Utm-Content": "1201",
    "X-Meta-Adset-Name": "Prospects",
    "X-Meta-Adset-Id": "2202",
    "X-Meta-Ad-Id": "3303",
    "X-Meta-Ad-Name": "%E5%B9%BF%E5%91%8A-A",
    "X-Meta-Placement": "instagram_story",
    "X-Meta-Site-Source-Name": "ig",
  });
});

// Facebook click aliases remain usable, while unresolved dynamic macros never become request metadata.
test("fbcli aliases fbclid and unresolved values are omitted", () => {
  assert.deepEqual(
    buildMetaAttributionHeaders(
      "?fbcli=legacy-click&ad_id=%7B%7Bad.id%7D%7D&ad_name=%20%20",
    ),
    { "X-Meta-Fbclid": "legacy-click" },
  );
});

// Long Unicode names must stay inside the Go server's total request-header budget.
test("encoded Meta header values are safely bounded", () => {
  const headers = buildMetaAttributionHeaders(
    `?ad_name=${encodeURIComponent("广告".repeat(1000))}`,
  );
  assert.ok(headers["X-Meta-Ad-Name"].length <= 768);
  assert.doesNotThrow(() => decodeURIComponent(headers["X-Meta-Ad-Name"]));
});
