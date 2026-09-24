import test from "node:test";
import assert from "node:assert/strict";
import { createCampaignVisibleTracker } from "../src/lib/timeSpent.js";

function fakeDocument(initial = "visible") {
  const listeners = new Map();
  return {
    visibilityState: initial,
    addEventListener(name, listener) { listeners.set(name, listener); },
    removeEventListener(name) { listeners.delete(name); },
    emit(name) { listeners.get(name)?.(); },
  };
}

function fakeWindow() {
  const listeners = new Map();
  return {
    addEventListener(name, listener) { listeners.set(name, listener); },
    removeEventListener(name) { listeners.delete(name); },
    emit(name) { listeners.get(name)?.(); },
  };
}

test("campaign visible tracker reports every interval and pauses while hidden", async () => {
  let milliseconds = 0;
  let interval;
  const documentRef = fakeDocument();
  const windowRef = fakeWindow();
  const reports = [];
  const tracker = createCampaignVisibleTracker({
    documentRef,
    windowRef,
    now: () => milliseconds,
    schedule: (callback) => { interval = callback; return 1; },
    cancel: () => {},
    report: async (seconds, options) => { reports.push({ seconds, options }); return { visible_seconds: seconds }; },
  });

  milliseconds = 10_000;
  await interval();
  documentRef.visibilityState = "hidden";
  milliseconds = 15_000;
  documentRef.emit("visibilitychange");
  await tracker.whenIdle();
  milliseconds = 45_000;
  await interval();
  documentRef.visibilityState = "visible";
  documentRef.emit("visibilitychange");
  milliseconds = 50_000;
  await interval();

  assert.deepEqual(reports.map(({ seconds }) => seconds), [10, 15, 20]);
  assert.equal(reports[1].options.useBeacon, true);
  tracker.cleanup();
});

test("campaign visible tracker sends the latest visible total on pagehide", async () => {
  let milliseconds = 0;
  const documentRef = fakeDocument();
  const windowRef = fakeWindow();
  const reports = [];
  const tracker = createCampaignVisibleTracker({
    documentRef,
    windowRef,
    now: () => milliseconds,
    schedule: () => 1,
    cancel: () => {},
    report: async (seconds, options) => { reports.push({ seconds, options }); return { visible_seconds: seconds }; },
  });
  milliseconds = 7_000;
  windowRef.emit("pagehide");
  await tracker.whenIdle();
  assert.deepEqual(reports, [{ seconds: 7, options: { useBeacon: true } }]);
  tracker.cleanup();
});
