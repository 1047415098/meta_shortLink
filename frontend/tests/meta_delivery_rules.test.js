import test from "node:test";
import assert from "node:assert/strict";
import {
  credentialStatus,
  pixelForm,
  pixelPayload,
  pixelTogglePayload,
  pixelSelectable,
  pixelUnavailableReason,
} from "../src/utils/meta.js";

// Pixel forms expose independent manual/automatic controls and no expiry setting.
test("Pixel form separates consultation rules and removes token expiry", () => {
  const form = pixelForm({
    manual_event_name: "WhatsAppConsultClick",
    token_expires_at: "2020-01-01T23:59:59Z",
  });
  assert.equal(form.manual_enabled, true);
  assert.equal(form.auto_enabled, true);
  assert.equal("manual_event_name" in form, false);
  assert.equal("token_expires_at" in form, false);
  assert.equal("clear_capi_token" in form, false);

  form.manual_enabled = false;
  form.auto_enabled = true;
  // Legacy callers cannot reintroduce credential deletion through the payload.
  form.clear_capi_token = true;
  const payload = pixelPayload(form, false);
  assert.equal(payload.manual_enabled, false);
  assert.equal(payload.auto_enabled, true);
  assert.equal("manual_event_name" in payload, false);
  assert.equal("token_expires_at" in payload, false);
  assert.equal("clear_capi_token" in payload, false);
});

test("Pixel list toggle changes only the master enabled state", () => {
  // A quick pause must preserve the token and each independent event rule.
  assert.deepEqual(pixelTogglePayload({ enabled: true }), { enabled: false });
  assert.deepEqual(pixelTogglePayload({ enabled: false }), { enabled: true });
});

// Legacy expiry labels cannot disable a permanent CAPI token after the feature is removed.
test("legacy expired status no longer blocks a configured Pixel", () => {
  const pixel = {
    enabled: true,
    has_capi_token: true,
    credential_status: "expired",
    token_expires_at: "2020-01-01T23:59:59Z",
  };
  assert.equal(pixelSelectable(pixel), true);
  assert.equal(pixelUnavailableReason(pixel), "");
  assert.deepEqual(credentialStatus("expired"), {
    label: "未验证",
    type: "info",
  });
});
