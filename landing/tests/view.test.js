import test from "node:test";
import assert from "node:assert/strict";
import * as landingView from "../src/lib/view.js";

const { reportLandingView } = landingView;

test("landing view module exposes browser Meta Pixel installation", () => {
  // Browser and server PageView delivery need one shared module so their
  // event identifiers cannot drift between independent call sites.
  assert.equal(typeof landingView.installMetaPixel, "function");
});

test("browser Meta Pixel loads the selected Pixel and shares the CAPI PageView event ID", () => {
  const inserted = [];
  const firstScript = {
    parentNode: {
      insertBefore(node, before) {
        inserted.push({ node, before });
      },
    },
  };
  const document = {
    createElement: () => ({}),
    getElementById: () => null,
    getElementsByTagName: () => [firstScript],
  };
  const scope = {};
  const state = {};

  assert.equal(
    landingView.installMetaPixel({
      pixelId: "1066571352827370",
      eventId: "wa_visit-1_view",
      document,
      scope,
      state,
    }),
    true,
  );
  assert.equal(inserted.length, 1);
  assert.equal(inserted[0].node.id, "meta-pixel-script");
  assert.equal(
    inserted[0].node.src,
    "https://connect.facebook.net/en_US/fbevents.js",
  );
  assert.deepEqual(scope.fbq.queue.map((args) => [...args]), [
    ["init", "1066571352827370"],
    ["track", "PageView", {}, { eventID: "wa_visit-1_view" }],
  ]);

  // Repeated Vue mounts must not create a second browser PageView.
  assert.equal(
    landingView.installMetaPixel({
      pixelId: "1066571352827370",
      eventId: "wa_visit-1_view",
      document,
      scope,
      state,
    }),
    false,
  );
  assert.equal(scope.fbq.queue.length, 2);
});

test("browser Meta Pixel stays disabled without a bound Pixel and event", () => {
  const scope = {};
  const document = {
    createElement: () => {
      throw new Error("loader must not be created");
    },
  };
  assert.equal(
    landingView.installMetaPixel({
      pixelId: "",
      eventId: "",
      document,
      scope,
      state: {},
    }),
    false,
  );
  assert.equal(scope.fbq, undefined);
});

test("browser Meta Pixel initializes for manual consultation when PageView is disabled", () => {
  const inserted = [];
  const firstScript = {
    parentNode: {
      insertBefore(node) {
        inserted.push(node);
      },
    },
  };
  const document = {
    createElement: () => ({}),
    getElementById: () => null,
    getElementsByTagName: () => [firstScript],
  };
  const scope = {};

  // A manual-only Pixel still needs fbevents.js and init, but must not invent a PageView.
  assert.equal(
    landingView.installMetaPixel({
      pixelId: "1066571352827370",
      eventId: "",
      document,
      scope,
      state: {},
    }),
    true,
  );
  assert.equal(inserted.length, 1);
  assert.deepEqual(scope.fbq.queue.map((args) => [...args]), [
    ["init", "1066571352827370"],
  ]);
});

test("manual consultation sends one browser AddToCart with the shared CAPI event ID", () => {
  const calls = [];
  const scope = { fbq: (...args) => calls.push(args) };
  const state = {};

  // The shared visit-scoped ID lets Meta deduplicate this browser event and CAPI.
  assert.equal(
    landingView.trackMetaConsult({
      eventId: "wa_visit-1_manual",
      scope,
      state,
    }),
    true,
  );
  assert.equal(
    landingView.trackMetaConsult({
      eventId: "wa_visit-1_manual",
      scope,
      state,
    }),
    false,
  );
  assert.deepEqual(calls, [
    ["track", "AddToCart", {}, { eventID: "wa_visit-1_manual" }],
  ]);
});

test("signed landing view posts once per document", async () => {
  const calls = [];
  const state = {};
  const request = async (...args) => calls.push(args);
  await reportLandingView({
    code: "hello world",
    ticket: "signed",
    request,
    state,
    // The business request must mirror the attribution captured by the entry URL.
    attributionHeaders: { "X-Meta-Ad-Id": "3303" },
  });
  await reportLandingView({
    code: "hello world",
    ticket: "signed",
    request,
    state,
  });
  assert.deepEqual(calls, [
    [
      "/hello%20world/view",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          "X-Meta-Ad-Id": "3303",
        },
        body: "ticket=signed",
        keepalive: true,
      },
    ],
  ]);
});
test("landing view does nothing without a ticket", async () => {
  const calls = [];
  await reportLandingView({
    code: "x",
    ticket: "",
    request: async (...args) => calls.push(args),
    state: {},
  });
  assert.deepEqual(calls, []);
});
