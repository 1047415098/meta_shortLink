import test from "node:test";
import assert from "node:assert/strict";
import { readFile, stat } from "node:fs/promises";

const projectFile = (path) => new URL(`../${path}`, import.meta.url);
const heroPath = "landing-assets/images/hero-79e77a51693b3249.webp";

test("the critical hero uses a small versioned WebP and starts before JavaScript", async () => {
  // The first-screen image previously dominated mobile transfer size, so the
  // build contract keeps it compressed, versioned and explicitly preloaded.
  const [html, view, hero] = await Promise.all([
    readFile(projectFile("index.html"), "utf8"),
    readFile(projectFile("src/views/LandingView.vue"), "utf8"),
    stat(projectFile(`public/${heroPath}`)),
  ]);
  assert.ok(hero.size <= 120_000, `hero is ${hero.size} bytes`);
  assert.match(html, new RegExp(`rel="preload"[^>]+${heroPath}`));
  assert.match(view, new RegExp(heroPath, "g"));
  assert.doesNotMatch(view, /images\/hero\.jpg/);
});

test("the single-screen landing entry does not ship a client router", async () => {
  // Go already resolves the short-code route, leaving no browser-side
  // navigation responsibility for vue-router.
  const [entry, app, packageJSON] = await Promise.all([
    readFile(projectFile("src/main.js"), "utf8"),
    readFile(projectFile("src/App.vue"), "utf8"),
    readFile(projectFile("package.json"), "utf8"),
  ]);
  assert.doesNotMatch(entry, /vue-router|createRouter|createWebHistory/);
  assert.doesNotMatch(app, /RouterView|vue-router/);
  assert.equal(JSON.parse(packageJSON).dependencies["vue-router"], undefined);
});

test("below-fold product media yields decoding and rendering work", async () => {
  // Product cards remain available after the first screen without competing
  // with the primary WhatsApp action for initial rendering work.
  const view = await readFile(
    projectFile("src/views/LandingView.vue"),
    "utf8",
  );
  assert.match(view, /loading="lazy"[^>]+decoding="async"/);
  assert.match(view, /\.products-section\s*\{[^}]*content-visibility:\s*auto/s);
});
