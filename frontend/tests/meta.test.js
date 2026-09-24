import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import {
  connectionForm,
  connectionPayload,
  pixelForm,
  pixelPayload,
  buildMetaTrackingURL,
  credentialStatus,
  pixelSelectable,
  pixelsForConnection,
  connectionIDForPixel,
  pixelUnavailableReason,
  GRAPH_API_VERSION,
  META_URL_PARAMETERS,
  META_CONSULT_EVENT,
  metaEventLabel,
} from "../src/utils/meta.js";

test("Meta event labels keep manual and automatic AddToCart actions distinct", () => {
  // Meta receives one standard name while the stable event ID preserves the
  // trigger needed by operators reviewing the delivery log.
  assert.equal(META_CONSULT_EVENT, "AddToCart");
  assert.equal(metaEventLabel({ event_name: "PageView" }), "浏览事件");
  assert.equal(
    metaEventLabel({ event_name: "AddToCart", id: "wa_visit_manual" }),
    "手动咨询 · AddToCart",
  );
  assert.equal(
    metaEventLabel({ event_name: "AddToCart", id: "wa_visit_auto" }),
    "自动跳转 · AddToCart",
  );
  assert.equal(
    metaEventLabel({ event_name: "WhatsAppAutoRedirect" }),
    "自动跳转（历史）",
  );
  assert.equal(
    metaEventLabel({ event_name: "StartListening", audio_novel_id: 7 }),
    "语音小说 · 开始收听",
  );
  assert.equal(
    metaEventLabel({ event_name: "ViewContent", audio_novel_id: 7 }),
    "语音小说 · 播放达标",
  );
});

test("Meta event page keeps raw names while identifying audio novel events", async () => {
  const source = await readFile(
    new URL("../src/views/MetaEventsView.vue", import.meta.url),
    "utf8",
  );
  assert.match(source, /"StartListening"/);
  assert.match(source, /"ViewContent"/);
  assert.match(source, /audio_novel_id/);
  assert.match(source, /语音小说编号/);
});

test("Meta URL parameter template keeps every canonical advertising field", () => {
  // Operators copy one immutable template so separate ad creators cannot drift
  // back to ambiguous legacy UTM mappings.
  assert.equal(
    META_URL_PARAMETERS,
    "utm_source={{site_source_name}}&utm_medium=paid_social&utm_campaign={{campaign.name}}&utm_content={{ad.id}}&campaign_id={{campaign.id}}&campaign_name={{campaign.name}}&adset_id={{adset.id}}&adset_name={{adset.name}}&ad_id={{ad.id}}&ad_name={{ad.name}}&placement={{placement}}&site_source_name={{site_source_name}}",
  );
});

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
test("Pixel edit reuses the displayed credential without rewriting it", () => {
  // The detail endpoint may reveal the existing token to an authenticated
  // administrator, but an unchanged value must not be encrypted again.
  const form = pixelForm(
    { connection_id: 3, pixel_id: "998", name: "Main" },
    "saved-capi-token",
  );
  assert.equal(form.capi_token, "saved-capi-token");
  assert.equal("capi_token" in pixelPayload(form, true), false);
  form.capi_token = "replacement-capi-token";
  assert.equal(pixelPayload(form, true).capi_token, "replacement-capi-token");
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
