function safeRate(value) {
  const rate = Number(value);
  return Number.isFinite(rate) && rate > 0 ? Math.min(rate, 4) : 1;
}

function roundedMedia(seconds) {
  return Math.round(Math.max(0, seconds) * 1000) / 1000;
}

function laterSnapshot(current, candidate) {
  if (!current) return candidate;
  return {
    playback_seconds: Math.max(current.playback_seconds, candidate.playback_seconds),
    media_consumed_seconds: Math.max(current.media_consumed_seconds, candidate.media_consumed_seconds),
  };
}

export function createPlaybackTracker({
  now = () => performance.now(),
  report = async () => null,
  currentPlaybackRate = () => 1,
} = {}) {
  // 所有累计值集中在一个状态对象，避免音频事件交错时出现多套计时来源。
  const state = {
    active: false,
    segmentStartedAt: null,
    playbackSeconds: 0,
    mediaConsumedSeconds: 0,
    confirmedPlaybackSeconds: 0,
    reportInFlight: false,
    pendingReport: null,
  };
  let segmentRate = 1;
  let resumeAfterSeek = false;
  let confirmedMediaSeconds = 0;
  let lastSubmitted = { playback_seconds: 0, media_consumed_seconds: 0 };
  let activeReport = Promise.resolve(false);

  function settle() {
    if (!state.active || state.segmentStartedAt === null) return;
    const current = now();
    const elapsed = Math.max(0, current - state.segmentStartedAt) / 1000;
    state.playbackSeconds += elapsed;
    state.mediaConsumedSeconds += elapsed * segmentRate;
    state.segmentStartedAt = current;
  }

  function stop() {
    if (!state.active) return;
    settle();
    state.active = false;
    state.segmentStartedAt = null;
  }

  function playing() {
    if (state.active) return;
    state.active = true;
    state.segmentStartedAt = now();
    segmentRate = safeRate(currentPlaybackRate());
  }

  function rateChanged() {
    // ratechange fires after the media element exposes its new value, so first
    // settle the elapsed segment with the old rate and only then switch rates.
    if (state.active) settle();
    segmentRate = safeRate(currentPlaybackRate());
  }

  function snapshot() {
    settle();
    return {
      playback_seconds: Math.max(0, Math.floor(state.playbackSeconds)),
      media_consumed_seconds: roundedMedia(state.mediaConsumedSeconds),
    };
  }

  function rememberConfirmation(result) {
    const playback = Number(result?.playback_seconds);
    const media = Number(result?.media_consumed_seconds);
    if (Number.isFinite(playback)) state.confirmedPlaybackSeconds = Math.max(state.confirmedPlaybackSeconds, playback);
    if (Number.isFinite(media)) confirmedMediaSeconds = Math.max(confirmedMediaSeconds, media);
  }

  function hasUnconfirmedProgress(payload) {
    return payload.playback_seconds > state.confirmedPlaybackSeconds || payload.media_consumed_seconds > confirmedMediaSeconds;
  }

  function submit(payload) {
    state.reportInFlight = true;
    lastSubmitted = laterSnapshot(lastSubmitted, payload);
    activeReport = Promise.resolve()
      .then(() => report(payload, { useBeacon: false }))
      .then((result) => {
        rememberConfirmation(result);
        return result;
      })
      .catch(() => false)
      .finally(() => {
        state.reportInFlight = false;
        const pending = state.pendingReport;
        state.pendingReport = null;
        if (pending && hasUnconfirmedProgress(pending)) submit(pending);
      });
    return activeReport;
  }

  async function flush({ useBeacon = false } = {}) {
    const payload = snapshot();
    if (payload.playback_seconds < lastSubmitted.playback_seconds || payload.media_consumed_seconds < lastSubmitted.media_consumed_seconds) return false;
    if (!hasUnconfirmedProgress(payload)) return false;
    if (useBeacon) {
      // 页面离开时不能等待正在进行的 fetch；Beacon 仍提交同一组单调累计值。
      lastSubmitted = laterSnapshot(lastSubmitted, payload);
      try {
        return await report(payload, { useBeacon: true });
      } catch {
        return false;
      }
    }
    if (state.reportInFlight) {
      state.pendingReport = laterSnapshot(state.pendingReport, payload);
      return false;
    }
    return submit(payload);
  }

  async function whenIdle() {
    while (state.reportInFlight || state.pendingReport) await activeReport;
  }

  return {
    state,
    playing,
    pause: stop,
    waiting: stop,
    stalled: stop,
    rateChanged,
    ended() { resumeAfterSeek = false; stop(); },
    seeking() { resumeAfterSeek = state.active; stop(); },
    seeked(media = {}) {
      const resume = resumeAfterSeek && media.paused === false && media.ended === false;
      resumeAfterSeek = false;
      if (resume) playing();
    },
    snapshot,
    flush,
    whenIdle,
  };
}
