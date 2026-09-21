import test from "node:test";
import assert from "node:assert/strict";
import {
  createVisibleTimeTracker,
  formatVisibleTime,
  reportTimeSpent,
  trackMetaTimeSpent,
} from "../src/lib/timeSpent.js";

function fakeDocument() {
  const listeners = new Map();
  return {
    visibilityState: "visible",
    documentElement: { dataset: {} },
    addEventListener(name, fn) { listeners.set(name, fn); },
    removeEventListener(name) { listeners.delete(name); },
    change(value) { this.visibilityState = value; listeners.get("visibilitychange")?.(); },
  };
}

test("visible timer pauses while the document is hidden and triggers once", () => {
  const documentRef = fakeDocument();
  let now = 0;
  let tick;
  const seconds = [];
  let reached = 0;
  const stop = createVisibleTimeTracker({
    threshold: 5,
    documentRef,
    now: () => now,
    schedule: (fn) => { tick = fn; return 1; },
    cancel: () => {},
    onTick: (value) => seconds.push(value),
    onThreshold: () => { reached += 1; },
  });
  now = 3000;
  tick();
  documentRef.change("hidden");
  now = 9000;
  tick();
  documentRef.change("visible");
  now = 11000;
  tick();
  now = 15000;
  tick();
  stop();
  assert.deepEqual(seconds, [0, 3, 5, 9]);
  assert.equal(reached, 1);
});

test("TimeSpent helpers format and deduplicate browser/server delivery", async () => {
  assert.equal(formatVisibleTime(65), "01:05");
  const calls = [];
  const scope = { fbq: (...args) => calls.push(args) };
  const state = {};
  assert.equal(trackMetaTimeSpent({ eventId: "wa_visit_time_spent", scope, state }), true);
  assert.equal(trackMetaTimeSpent({ eventId: "wa_visit_time_spent", scope, state }), false);
  const requests = [];
  await reportTimeSpent({ code: "hello", ticket: "signed", state, request: async (...args) => requests.push(args) });
  await reportTimeSpent({ code: "hello", ticket: "signed", state, request: async (...args) => requests.push(args) });
  assert.deepEqual(calls, [["trackCustom", "TimeSpent", {}, { eventID: "wa_visit_time_spent" }]]);
  assert.equal(requests.length, 1);
  assert.equal(requests[0][0], "/hello/time-spent");
});
