import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const projectFile = (path) => new URL(`../${path}`, import.meta.url);
const anyTrackModule = await import("../src/lib/anytrack.js").catch(() => ({}));

test("manual consultation reports one AnyTrack event", () => {
  const calls = [];

  assert.equal(typeof anyTrackModule.trackAnyTrackManualContact, "function");

  const reported = anyTrackModule.trackAnyTrackManualContact("manual", (...args) => {
    calls.push(args);
  });

  assert.equal(reported, true);
  assert.deepEqual(calls, [
    [
      "trigger",
      "WhatsAppChatClick",
      { label: "manual_whatsapp_contact" },
    ],
  ]);
});

test("automatic redirect never reports a chat-click event", () => {
  const calls = [];

  assert.equal(typeof anyTrackModule.trackAnyTrackManualContact, "function");

  const reported = anyTrackModule.trackAnyTrackManualContact("auto", (...args) => {
    calls.push(args);
  });

  assert.equal(reported, false);
  assert.deepEqual(calls, []);
});

test("landing loads the property tag and excludes its internal form from automatic tracking", async () => {
  const [html, loader, view] = await Promise.all([
    readFile(projectFile("index.html"), "utf8"),
    readFile(
      projectFile("public/landing-assets/anytrack-loader.js"),
      "utf8",
    ),
    readFile(projectFile("src/views/LandingView.vue"), "utf8"),
  ]);

  assert.match(html, /landing-assets\/anytrack-loader\.js/);
  assert.match(loader, /assets\.anytrack\.io\/BZgLiRKLRvlt\.js/);
  assert.match(view, /id="contact-form"[^>]+class="at-do-not-track"/);
});
