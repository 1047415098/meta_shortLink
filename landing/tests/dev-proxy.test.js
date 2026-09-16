import test from "node:test";
import assert from "node:assert/strict";
import {
  extractLandingBootstrap,
  isLandingPagePath,
} from "../dev/bootstrapProxy.js";

test("landing development proxy only intercepts public short-code pages", () => {
  // Assets and action endpoints must continue through Vite or its API proxy.
  assert.equal(isLandingPagePath("/hello?utm_source=facebook"), true);
  assert.equal(isLandingPagePath("/abc_123"), true);
  assert.equal(isLandingPagePath("/hello/contact"), false);
  assert.equal(isLandingPagePath("/landing-assets/image.webp"), false);
  assert.equal(isLandingPagePath("/admin/login"), false);
});

test("landing development proxy extracts the signed bootstrap script", () => {
  const script = extractLandingBootstrap(
    '<html><script id="landing-data" type="application/json" nonce="n">{"ticket":"signed"}</script></html>',
  );
  assert.match(script, /"ticket":"signed"/);
  assert.throws(
    () => extractLandingBootstrap("<html>missing</html>"),
    /bootstrap/i,
  );
});
