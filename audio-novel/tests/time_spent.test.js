import test from "node:test";
import assert from "node:assert/strict";
import { reportTimeSpent } from "../src/lib/timeSpent.js";

test("audio novel TimeSpent posts to the literature surface once", async () => {
  const calls = [];
  const state = {};
  await reportTimeSpent({ code: "hello world", ticket: "signed", state, request: async (...args) => calls.push(args) });
  await reportTimeSpent({ code: "hello world", ticket: "signed", state, request: async (...args) => calls.push(args) });
  assert.equal(calls.length, 1);
  assert.equal(calls[0][0], "/audio-novel/hello%20world/time-spent");
  assert.equal(calls[0][1].body, "ticket=signed");
});
