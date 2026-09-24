import test from "node:test";
import assert from "node:assert/strict";
import { createPlaybackTracker } from "../src/lib/playback.js";

function createClock() {
  let value = 0;
  return {
    now: () => value,
    advance(seconds) { value += seconds * 1000; },
  };
}

test("playback tracker counts only active wall time", () => {
  const clock = createClock();
  const tracker = createPlaybackTracker({ now: clock.now });
  tracker.playing();
  clock.advance(4);
  tracker.waiting();
  clock.advance(3);
  tracker.playing();
  clock.advance(6);
  tracker.pause();
  assert.deepEqual(tracker.snapshot(), { playback_seconds: 10, media_consumed_seconds: 10 });
});

test("pagehide reporting does not stop an active playback segment", async () => {
  const clock = createClock();
  const reports = [];
  const tracker = createPlaybackTracker({
    now: clock.now,
    report: async (payload, options) => { reports.push({ payload, options }); return { playback_seconds: payload.playback_seconds }; },
  });
  tracker.playing();
  clock.advance(5);
  await tracker.flush({ useBeacon: true });
  clock.advance(5);
  tracker.pause();
  assert.equal(tracker.snapshot().playback_seconds, 10);
  assert.equal(reports[0].payload.playback_seconds, 5);
  assert.equal(reports[0].options.useBeacon, true);
});

test("pause, waiting, ended and seeking settle playback idempotently", () => {
  const clock = createClock();
  const tracker = createPlaybackTracker({ now: clock.now });
  tracker.playing();
  tracker.playing();
  clock.advance(3);
  tracker.seeking();
  clock.advance(20); // 跳转音频时间轴本身不计入实际收听时长。
  tracker.seeked({ paused: false, ended: false });
  clock.advance(2);
  tracker.pause();
  tracker.pause();
  clock.advance(8);
  tracker.ended();
  assert.equal(tracker.snapshot().playback_seconds, 5);

  tracker.playing();
  clock.advance(1);
  tracker.seeking();
  tracker.seeked({ paused: true, ended: false });
  clock.advance(4);
  assert.equal(tracker.snapshot().playback_seconds, 6);
});

test("stalled and ended settle an active segment while ended seek never resumes", () => {
  const clock = createClock();
  const tracker = createPlaybackTracker({ now: clock.now });
  tracker.playing();
  clock.advance(2);
  tracker.stalled();
  clock.advance(5);
  tracker.playing();
  clock.advance(3);
  tracker.seeking();
  tracker.seeked({ paused: false, ended: true });
  clock.advance(4);
  tracker.playing();
  clock.advance(1);
  tracker.ended();
  clock.advance(9);
  assert.equal(tracker.snapshot().playback_seconds, 6);
});

test("media consumption follows playback rate while wall time stays real", () => {
  const clock = createClock();
  const tracker = createPlaybackTracker({ now: clock.now, currentPlaybackRate: () => 2 });
  tracker.playing();
  clock.advance(1);
  tracker.pause();
  assert.deepEqual(tracker.snapshot(), { playback_seconds: 1, media_consumed_seconds: 2 });
});

test("rate changes settle the active segment before using the new safe rate", () => {
  const clock = createClock();
  let rate = 1;
  const tracker = createPlaybackTracker({ now: clock.now, currentPlaybackRate: () => rate });
  tracker.playing();
  clock.advance(2);
  rate = 2;
  tracker.rateChanged();
  clock.advance(3);
  tracker.pause();
  assert.deepEqual(tracker.snapshot(), { playback_seconds: 5, media_consumed_seconds: 8 });

  // A paused rate change must not count the time spent away from playback.
  clock.advance(100);
  rate = 8;
  tracker.rateChanged();
  tracker.playing();
  clock.advance(1);
  tracker.pause();
  assert.deepEqual(tracker.snapshot(), { playback_seconds: 6, media_consumed_seconds: 12 });
});

test("overlapping interval reports coalesce and never submit a lower cumulative value", async () => {
  const clock = createClock();
  const reports = [];
  let releaseFirst;
  const first = new Promise((resolve) => { releaseFirst = resolve; });
  const tracker = createPlaybackTracker({
    now: clock.now,
    report: async (payload) => {
      reports.push(payload);
      if (reports.length === 1) await first;
      return { playback_seconds: payload.playback_seconds };
    },
  });
  tracker.playing();
  clock.advance(10);
  const firstFlush = tracker.flush();
  clock.advance(5);
  await tracker.flush();
  releaseFirst();
  await firstFlush;
  await tracker.whenIdle();
  assert.deepEqual(reports.map((item) => item.playback_seconds), [10, 15]);
  assert.equal(tracker.state.confirmedPlaybackSeconds, 15);
});

test("a clipped or failed report can retry without ever decreasing counters", async () => {
  const clock = createClock();
  const reports = [];
  let attempt = 0;
  const tracker = createPlaybackTracker({
    now: clock.now,
    currentPlaybackRate: () => 2,
    report: async (payload) => {
      reports.push(payload);
      attempt += 1;
      if (attempt === 1) throw new Error("offline");
      if (attempt === 2) return { playback_seconds: 8, media_consumed_seconds: 16 };
      return { playback_seconds: payload.playback_seconds, media_consumed_seconds: payload.media_consumed_seconds };
    },
  });
  tracker.playing();
  clock.advance(10);
  await tracker.flush();
  await tracker.flush();
  clock.advance(1);
  await tracker.flush();
  assert.deepEqual(reports.map((item) => [item.playback_seconds, item.media_consumed_seconds]), [[10, 20], [10, 20], [11, 22]]);
});
