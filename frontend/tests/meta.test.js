import test from "node:test";
import assert from "node:assert/strict";
import {
  connectionForm,
  connectionPayload,
  pixelForm,
  pixelPayload,
  buildMetaTrackingURL,
  credentialStatus,
  pixelSelectable,
  pixelsForConnection,
  preferredPixelID,
  connectionIDForPixel,
  pixelUnavailableReason,
  GRAPH_API_VERSION,
} from "../src/utils/meta.js";

test("Meta account payload contains only Pixel grouping fields", () => {
  // Removed Insights inputs must not survive in browser state or reach the
  // account API when an old response or caller still supplies them.
  const form = connectionForm({
    name: "Account",
    account_id: "123",
    api_version: "v99.0",
    insights_enabled: true,
    read_token: "must-not-copy",
    report_time: "conversion",
  });
  assert.deepEqual(connectionPayload(form, false), {
    name: "Account",
    account_id: "123",
  });
  // Client payloads cannot override the system Graph contract.
  assert.equal("api_version" in form, false);
  assert.equal(GRAPH_API_VERSION, "v26.0");
  assert.equal("read_token" in form, false);
  assert.equal("insights_enabled" in form, false);
});

test("Pixel edits omit immutable IDs and blank credentials", () => {
  const payload = pixelPayload(
    pixelForm({ connection_id: 3, pixel_id: "998", name: "Main" }),
    true,
  );
  assert.equal("connection_id" in payload, false);
  assert.equal("pixel_id" in payload, false);
  assert.equal("capi_token" in payload, false);
  // Event names are fixed by the backend; the form sends only independent rules.
  assert.equal("manual_event_name" in payload, false);
  assert.equal(payload.auto_enabled, true);
});
test("a new Pixel starts with qualified PageView and consultation delivery enabled", () => {
  // A saved Pixel is ready for the complete landing funnel without requiring
  // a second edit after the administrator supplies its CAPI token.
  const form = pixelForm();
  assert.equal(form.enabled, true);
  assert.equal(form.pageview_enabled, true);
  assert.equal(form.manual_enabled, true);
  assert.equal(form.auto_enabled, true);
});

test("link Pixel choices stay inside the selected account and reject unusable targets", () => {
  // Link creation must never route an event to another account or to a target
  // that cannot authenticate a CAPI request.
  const pixels = [
    { id: 1, connection_id: 7, enabled: true, has_capi_token: true },
    { id: 2, connection_id: 7, enabled: false, has_capi_token: true },
    { id: 3, connection_id: 8, enabled: true, has_capi_token: true },
    { id: 4, connection_id: 7, enabled: true, has_capi_token: false },
    {
      id: 5,
      connection_id: 7,
      enabled: true,
      has_capi_token: true,
      credential_status: "expired",
    },
  ];
  assert.deepEqual(
    pixelsForConnection(pixels, 7).map((pixel) => pixel.id),
    [1, 2, 4, 5],
  );
  assert.equal(pixelSelectable(pixels[0]), true);
  assert.equal(pixelSelectable(pixels[1]), false);
  assert.equal(pixelSelectable(pixels[3]), false);
  // The removed manual expiry state no longer disables a configured token.
  assert.equal(pixelSelectable(pixels[4]), true);
  assert.equal(pixelUnavailableReason(pixels[1]), "已停用");
  assert.equal(pixelUnavailableReason(pixels[3]), "未保存 CAPI Token");
  assert.equal(pixelUnavailableReason(pixels[4]), "");
});

test("a single usable Pixel is selected automatically but multiple targets require a choice", () => {
  // Automatic selection is safe only when the account has one eligible target.
  const one = [
    { id: 11, connection_id: 7, enabled: true, has_capi_token: true },
    { id: 12, connection_id: 7, enabled: false, has_capi_token: true },
  ];
  assert.equal(preferredPixelID(one, 7), 11);
  assert.equal(
    preferredPixelID(
      [
        ...one,
        { id: 13, connection_id: 7, enabled: true, has_capi_token: true },
      ],
      7,
    ),
    null,
  );
  assert.equal(preferredPixelID(one, 7, 11), 11);
  assert.equal(preferredPixelID(one, 8, 11), null);
});
test("selecting a Pixel derives its owning account for the saved short link", () => {
  // The operator chooses one destination while the persisted link still keeps
  // the account/Pixel pair required by the database relationship.
  const pixels = [
    { id: 21, connection_id: 7 },
    { id: 22, connection_id: 8 },
  ];
  assert.equal(connectionIDForPixel(pixels, 22), 8);
  assert.equal(connectionIDForPixel(pixels, 999), null);
  assert.equal(connectionIDForPixel(pixels, null), null);
});
test("tracking URL uses explicit Meta IDs and never includes a token", () => {
  const value = buildMetaTrackingURL("https://example.com/sale", {
    campaign_id: "cmp 1",
    adset_id: "set/2",
    ad_id: "ad3",
    utm_source: "facebook",
    site_source_name: "{{site_source_name}}",
    token: "secret",
  });
  assert.equal(
    value,
    "https://example.com/sale?utm_source=facebook&site_source_name=%7B%7Bsite_source_name%7D%7D&campaign_id=cmp+1&adset_id=set%2F2&ad_id=ad3",
  );
  assert.equal(value.includes("secret"), false);
});
test("credential status maps validity to clear Chinese labels and colors", () => {
  assert.deepEqual(credentialStatus("valid"), {
    label: "有效",
    type: "success",
  });
  assert.deepEqual(credentialStatus("unverified"), {
    label: "未验证",
    type: "info",
  });
  assert.deepEqual(credentialStatus("invalid"), {
    label: "无效",
    type: "danger",
  });
  assert.deepEqual(credentialStatus("expired"), {
    label: "未验证",
    type: "info",
  });
});
test("tracking URL rejects embedded credentials without echoing secrets", () => {
  for (const base of [
    "https://example.com/sale?access_token=secret",
    "https://example.com/sale?client_secret=synthetic-value",
    "https://user:pass@example.com/sale",
  ]) {
    assert.throws(
      () => buildMetaTrackingURL(base, { utm_source: "facebook" }),
      (error) =>
        error.message === "链接包含凭证信息，请先移除后重试" &&
        !error.message.includes("secret") &&
        !error.message.includes("pass"),
    );
  }
  assert.equal(
    buildMetaTrackingURL("https://example.com/sale?utm_medium=paid", {
      utm_source: "facebook",
    }),
    "https://example.com/sale?utm_medium=paid&utm_source=facebook",
  );
});
test("missing credential status is presented as unconfigured", () => {
  assert.deepEqual(credentialStatus("missing"), {
    label: "未配置",
    type: "info",
  });
});
