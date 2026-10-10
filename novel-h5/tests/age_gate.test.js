import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

test("age confirmation and countdown are temporarily bypassed for novel short links", async () => {
  // The temporary business setting must let campaign visitors enter without the 18+ overlay.
  const app = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");

  assert.match(app, /const ageGatePassed\s*=\s*ref\(true\)/);
  assert.doesNotMatch(app, /import AgeGateView/);
  assert.doesNotMatch(app, /<AgeGateView/);
  assert.doesNotMatch(app, /function completeAgeGate/);
});

test("free novel keeps only the countdown navigation path", async () => {
  const app = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");

  assert.match(app, /const ageGatePassed\s*=\s*ref\(true\)/);
  assert.match(app, /if\(ageGatePassed\.value\)startNovelExperience\(\)/);
  assert.match(app, /<RouterView\s*\/>/);
  assert.doesNotMatch(app, /PhotoWallStartup|photoWallOpen/);
  assert.match(app, /if\s*\(bootstrap\.ticket\)\s*fetch\([^\n]+\/view/);
});

test("age gate slider supports touch pointer and keyboard confirmation", async () => {
  const component = await readFile(new URL("../src/components/AgeGateView.vue", import.meta.url), "utf8");

  assert.match(component, /@pointerdown="startSlider"/);
  assert.match(component, /@pointermove="moveSlider"/);
  assert.match(component, /@pointerup="finishSlider"/);
  assert.match(component, /@keydown="onSliderKeydown"/);
  assert.match(component, /if\s*\(progress\.value\s*>=\s*90\)\s*beginCountdown\(\)/);
  assert.match(component, /setInterval\(updateCountdown,\s*200\)/);
  assert.match(component, /emit\("complete"\)/);
});
