import test from "node:test";
import assert from "node:assert/strict";
import { reportLandingView } from "../src/lib/view.js";

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
