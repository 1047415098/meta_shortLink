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

test("campaign navigation starts directly after the bootstrap request", async () => {
  // With the gate off, the normal RouterView is rendered after an unavailable-state check.
  const app = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");

  assert.match(app, /const ageGatePassed\s*=\s*ref\(true\)/);
  assert.match(app, /if\s*\(ageGatePassed\.value\)\s*startNovelExperience\(\)/);
  assert.match(app, /<RouterView v-else\s*\/>/);
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
