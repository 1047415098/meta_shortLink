import test from "node:test";
import assert from "node:assert/strict";
import {
  createContactCountdown,
  submitLandingContact,
} from "../src/lib/contact.js";

function setup(delay = 3) {
  let time = 0;
  const ticks = new Map();
  const events = new EventTarget();
  const form = new EventTarget();
  const trigger = { value: "manual" };
  const submitted = [];
  let remaining;
  form.requestSubmit = () => {
    form.dispatchEvent(new Event("submit"));
    submitted.push(trigger.value);
  };
  const stop = createContactCountdown({
    form,
    trigger,
    delay,
    events,
    now: () => time,
    schedule: (fn) => {
      ticks.set(1, fn);
      return 1;
    },
    cancel: (id) => ticks.delete(id),
    onRemaining: (value) => {
      remaining = value;
    },
  });
  return {
    form,
    events,
    trigger,
    submitted,
    stop,
    ticks,
    remaining: () => remaining,
    advance(ms) {
      time += ms;
      for (const fn of [...ticks.values()]) fn();
    },
  };
}

test("deadline countdown submits exactly once as auto", () => {
  const state = setup();
  assert.equal(state.remaining(), 3);
  state.advance(1100);
  assert.equal(state.remaining(), 2);
  state.advance(1900);
  assert.deepEqual(state.submitted, ["auto"]);
  state.advance(10000);
  assert.deepEqual(state.submitted, ["auto"]);
});
test("manual submit overrides stale auto and cancels countdown", () => {
  const state = setup();
  state.trigger.value = "auto";
  state.form.requestSubmit();
  state.advance(5000);
  assert.deepEqual(state.submitted, ["manual"]);
  assert.equal(state.ticks.size, 0);
});
test("zero disables automatic consultation but keeps manual classification", () => {
  const state = setup(0);
  state.advance(9000);
  assert.deepEqual(state.submitted, []);
  assert.equal(state.ticks.size, 0);
  state.trigger.value = "auto";
  state.form.requestSubmit();
  assert.deepEqual(state.submitted, ["manual"]);
});
test("back cache restoration resets manual without restarting auto", () => {
  const state = setup();
  state.advance(3000);
  state.events.dispatchEvent(new Event("pagehide"));
  const restored = new Event("pageshow");
  Object.defineProperty(restored, "persisted", { value: true });
  state.events.dispatchEvent(restored);
  assert.equal(state.trigger.value, "manual");
  assert.equal(state.remaining(), null);
  state.advance(10000);
  assert.deepEqual(state.submitted, ["auto"]);
  state.form.requestSubmit();
  assert.deepEqual(state.submitted, ["auto", "manual"]);
});
test("pagehide cancels countdown before navigation", () => {
  const state = setup();
  state.events.dispatchEvent(new Event("pagehide"));
  state.advance(10000);
  assert.deepEqual(state.submitted, []);
  assert.equal(state.ticks.size, 0);
});
test("cleanup removes timers and event handlers", () => {
  const state = setup();
  state.stop();
  state.advance(10000);
  state.trigger.value = "sentinel";
  state.form.dispatchEvent(new Event("submit"));
  assert.equal(state.trigger.value, "sentinel");
  assert.deepEqual(state.submitted, []);
});
test("history restoration with a fresh document never restarts auto", () => {
  let scheduled = false;
  const form = new EventTarget();
  const trigger = { value: "auto" };
  const cleanup = createContactCountdown({
    form,
    trigger,
    delay: 3,
    restored: true,
    events: new EventTarget(),
    onRemaining: () => {},
    schedule: () => {
      scheduled = true;
    },
  });
  assert.equal(scheduled, false);
  form.dispatchEvent(new Event("submit"));
  assert.equal(trigger.value, "manual");
  cleanup();
});

// Fetch submission must expose the consultation request and navigate only after the server records it.
test("consultation posts attribution headers before navigating to WhatsApp", async () => {
  const calls = [];
  const navigations = [];
  await submitLandingContact({
    code: "hello world",
    ticket: "signed-ticket",
    trigger: "manual",
    attributionHeaders: { "X-Meta-Ad-Id": "3303" },
    request: async (...args) => {
      calls.push(args);
      return {
        ok: true,
        json: async () => ({ target_url: "https://wa.me/13365661092" }),
      };
    },
    navigate: (target) => navigations.push(target),
  });
  assert.deepEqual(calls, [
    [
      "/hello%20world/contact",
      {
        method: "POST",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/x-www-form-urlencoded",
          "X-Requested-With": "XMLHttpRequest",
          "X-Meta-Ad-Id": "3303",
        },
        body: "ticket=signed-ticket&trigger=manual",
      },
    ],
  ]);
  assert.deepEqual(navigations, ["https://wa.me/13365661092"]);
});

test("AnyTrack failure never blocks a recorded manual consultation", async () => {
  let trackingAttempts = 0;
  const navigations = [];

  await submitLandingContact({
    code: "hello",
    ticket: "signed-ticket",
    trigger: "manual",
    request: async () => ({
      ok: true,
      json: async () => ({ target_url: "https://wa.me/13365661092" }),
    }),
    anyTrack: () => {
      trackingAttempts += 1;
      throw new Error("tracking unavailable");
    },
    navigate: (target) => navigations.push(target),
  });

  assert.equal(trackingAttempts, 1);
  assert.deepEqual(navigations, ["https://wa.me/13365661092"]);
});

// The countdown listener owns classification and prevents native form navigation when Fetch is enabled.
test("consultation submit hook receives manual and automatic classifications", () => {
  const form = new EventTarget();
  const trigger = { value: "manual" };
  const submitted = [];
  form.requestSubmit = () => {
    const event = new Event("submit", { cancelable: true });
    form.dispatchEvent(event);
    return event.defaultPrevented;
  };
  const cleanup = createContactCountdown({
    form,
    trigger,
    delay: 0,
    events: new EventTarget(),
    onRemaining: () => {},
    onSubmit: (kind) => submitted.push(kind),
  });
  assert.equal(form.requestSubmit(), true);
  assert.deepEqual(submitted, ["manual"]);
  cleanup();
});
