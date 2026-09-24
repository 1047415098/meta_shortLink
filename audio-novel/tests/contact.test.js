import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { compile } from "@vue/compiler-dom";
import { parse } from "@vue/compiler-sfc";
import { createSSRApp } from "vue";
import * as Vue from "vue";
import { renderToString } from "@vue/server-renderer";
import { submitAudioNovelContact } from "../src/lib/contact.js";

test("audio novel consultation is manual and scoped to the current code", async () => {
  let captured;
  const data = await submitAudioNovelContact({
    code: "hello",
    ticket: "signed.ticket",
    request: async (url, options) => {
      captured = { url, options };
      return { ok: true, json: async () => ({ target_url: "https://wa.me/123" }) };
    }
  });
  assert.equal(captured.url, "/audio-novel/hello/contact");
  assert.equal(captured.options.body.get("trigger"), "manual");
  assert.equal(data.target_url, "https://wa.me/123");
});

async function renderContact(targetURL) {
  const source = await readFile(new URL("../src/components/WhatsAppAction.vue", import.meta.url), "utf8");
  const { descriptor } = parse(source);
  const { code } = compile(descriptor.template.content, { mode: "function" });
  const renderTemplate = new Function("Vue", code)(Vue);
  const state = {
    bootstrap: { link: { target_url: targetURL } },
    busy: false,
    message: "",
    openWhatsApp() {},
  };
  return renderToString(createSSRApp({ render: () => renderTemplate(state, []) }));
}

test("WhatsApp CTA is visible only when bootstrap exposes a target URL", async () => {
  const campaign = await renderContact("");
  const legacy = await renderContact("https://wa.me/12345678");
  assert.doesNotMatch(campaign, /Continue on WhatsApp/);
  assert.match(legacy, /Continue on WhatsApp/);
});
