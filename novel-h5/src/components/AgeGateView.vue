<script setup>
import { computed, onBeforeUnmount, ref } from "vue";
import { useI18n } from "vue-i18n";

const emit = defineEmits(["complete"]);
const { t } = useI18n();
const stage = ref("verify");
const progress = ref(0);
const seconds = ref(10);
const track = ref(null);
const dragging = ref(false);
const sliderOffset = computed(() => Math.max(0, (track.value?.clientWidth || 58) - 58) * progress.value / 100);
let countdownTimer = null;
let countdownEndsAt = 0;
let completed = false;
let activePointerId = null;

function beginCountdown() {
  if (stage.value !== "verify") return;
  stage.value = "countdown";
  countdownEndsAt = Date.now() + 10_000;
  updateCountdown();
  countdownTimer = window.setInterval(updateCountdown, 200);
}

function updateCountdown() {
  seconds.value = Math.max(0, Math.ceil((countdownEndsAt - Date.now()) / 1000));
  if (seconds.value === 0 && !completed) {
    completed = true;
    window.clearInterval(countdownTimer);
    countdownTimer = null;
    emit("complete");
  }
}

function moveSlider(event) {
  if (stage.value !== "verify" || !dragging.value || event.pointerId !== activePointerId) return;
  const bounds = track.value?.getBoundingClientRect();
  if (!bounds) return;
  const thumbSize = 52;
  const travel = Math.max(1, bounds.width - thumbSize);
  const offset = Math.max(0, Math.min(travel, event.clientX - bounds.left - thumbSize / 2));
  progress.value = Math.round((offset / travel) * 100);
}

function startSlider(event) {
  if (stage.value !== "verify") return;
  dragging.value = true;
  activePointerId = event.pointerId;
  event.currentTarget.setPointerCapture?.(event.pointerId);
  moveSlider(event);
}

function finishSlider() {
  if (stage.value !== "verify" || !dragging.value) return;
  dragging.value = false;
  activePointerId = null;
  if (progress.value >= 90) {
    beginCountdown();
    return;
  }
  progress.value = 0;
}

function onSliderKeydown(event) {
  if (stage.value !== "verify") return;
  if (event.key === "ArrowRight" || event.key === "ArrowUp") {
    event.preventDefault();
    progress.value = Math.min(100, progress.value + 10);
  } else if (event.key === "ArrowLeft" || event.key === "ArrowDown" || event.key === "Home") {
    event.preventDefault();
    progress.value = event.key === "Home" ? 0 : Math.max(0, progress.value - 10);
  } else if (event.key === "End") {
    event.preventDefault();
    progress.value = 100;
  }
  if (progress.value >= 90) beginCountdown();
}

function leaveSite() {
  if (window.history.length > 1) window.history.back();
  else window.location.replace("about:blank");
}

onBeforeUnmount(() => window.clearInterval(countdownTimer));
</script>

<template>
  <!-- 年龄确认和倒计时共用同一全屏遮罩，只在倒计时完成后放行小说页面。 -->
  <section class="age-gate" :aria-label="t('ageGateTitle')">
    <div class="age-gate-glow age-gate-glow-one" aria-hidden="true"></div>
    <div class="age-gate-glow age-gate-glow-two" aria-hidden="true"></div>

    <div class="age-gate-card" aria-live="polite">
      <div v-if="stage === 'verify'" class="age-gate-content">
        <div class="age-gate-mark" aria-hidden="true">18+</div>
        <p class="age-gate-eyebrow">{{ t("ageGateEyebrow") }}</p>
        <h1>{{ t("ageGateTitle") }}</h1>
        <p class="age-gate-description">{{ t("ageGateDescription") }}</p>
        <p class="age-gate-instruction">{{ t("ageGateInstruction") }}</p>

        <div ref="track" class="age-gate-track" @pointermove="moveSlider" @pointerup="finishSlider" @pointercancel="finishSlider">
          <div class="age-gate-fill" :style="{ width: `calc(58px + ${sliderOffset}px)` }"></div>
          <span class="age-gate-track-label">{{ t("ageGateInstruction") }}</span>
          <div
            class="age-gate-thumb"
            role="slider"
            tabindex="0"
            :aria-label="t('ageGateSliderLabel')"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-valuenow="progress"
            :aria-valuetext="`${progress}%`"
            :style="{ transform: `translateX(${sliderOffset}px)` }"
            @pointerdown="startSlider"
            @keydown="onSliderKeydown"
          >
            <i class="fa-solid fa-chevron-right" aria-hidden="true"></i>
          </div>
        </div>
        <p class="age-gate-keyboard-hint">{{ t("ageGateKeyboardHint") }}</p>
        <p class="age-gate-underage">{{ t("ageGateUnderage") }}</p>
        <button class="age-gate-leave" type="button" @click="leaveSite">{{ t("ageGateLeave") }}</button>
      </div>

      <div v-else class="age-gate-content age-gate-countdown">
        <div class="age-gate-success" aria-hidden="true"><i class="fa-solid fa-check"></i></div>
        <p class="age-gate-eyebrow">{{ t("ageGatePassed") }}</p>
        <h1>{{ t("ageGateWelcome") }}</h1>
        <div
          class="age-gate-countdown-ring"
          role="timer"
          :aria-label="t('ageGateEntering', { sec: seconds })"
        >
          <svg viewBox="0 0 88 88" aria-hidden="true">
            <circle class="age-gate-ring-track" cx="44" cy="44" r="37"></circle>
            <circle
              class="age-gate-ring-progress"
              cx="44"
              cy="44"
              r="37"
              :style="{ strokeDashoffset: `${232.5 * (seconds / 10)}` }"
            ></circle>
          </svg>
          <span>{{ seconds }}</span>
        </div>
        <p class="age-gate-countdown-label">{{ t("ageGateEntering", { sec: seconds }) }}</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.age-gate {
  position: fixed;
  z-index: 1000;
  inset: 0;
  min-height: 100svh;
  height: 100dvh;
  padding: max(24px, env(safe-area-inset-top)) 20px max(24px, env(safe-area-inset-bottom));
  overflow: hidden;
  display: grid;
  place-items: center;
  color: #24262b;
  background: radial-gradient(circle at 15% 12%, #fff1e8 0, transparent 34%),
    radial-gradient(circle at 85% 88%, #ffe6d1 0, transparent 32%), #f3f4f6;
  font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
.age-gate-glow {
  position: absolute;
  width: 220px;
  aspect-ratio: 1;
  border-radius: 50%;
  filter: blur(70px);
  opacity: 0.38;
  pointer-events: none;
}
.age-gate-glow-one { top: 7%; left: -90px; background: #ff8d67; }
.age-gate-glow-two { right: -110px; bottom: 5%; background: #ffc27e; }
.age-gate-card {
  position: relative;
  width: min(100%, 420px);
  padding: 38px 30px 30px;
  border: 1px solid rgba(255, 107, 74, 0.12);
  border-radius: 28px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 28px 80px rgba(53, 43, 37, 0.14), 0 8px 24px rgba(255, 107, 74, 0.08);
  -webkit-backdrop-filter: blur(18px);
  backdrop-filter: blur(18px);
}
.age-gate-content { display: flex; flex-direction: column; align-items: center; text-align: center; }
.age-gate-mark {
  display: grid;
  width: 74px;
  height: 74px;
  margin-bottom: 18px;
  place-items: center;
  border: 1px solid rgba(255, 255, 255, 0.56);
  border-radius: 24px;
  color: #fff;
  background: linear-gradient(145deg, #ff8765, #ff6346);
  box-shadow: 0 12px 26px rgba(255, 107, 74, 0.28);
  font-size: 23px;
  font-weight: 850;
  letter-spacing: -1px;
}
.age-gate-eyebrow { margin: 0 0 8px; color: #ed684a; font-size: 12px; font-weight: 800; letter-spacing: 0.12em; text-transform: uppercase; }
.age-gate h1 { margin: 0; font-size: clamp(23px, 6vw, 29px); line-height: 1.22; letter-spacing: -0.04em; }
.age-gate-description { max-width: 310px; margin: 12px 0 0; color: #72777f; font-size: 14px; line-height: 1.6; }
.age-gate-instruction { margin: 26px 0 12px; font-size: 14px; font-weight: 750; }
.age-gate-track {
  position: relative;
  width: 100%;
  height: 60px;
  padding: 4px;
  overflow: hidden;
  border: 1px solid #e8e9ec;
  border-radius: 999px;
  background: #f1f2f4;
  touch-action: none;
  user-select: none;
}
.age-gate-fill { position: absolute; inset: 0 auto 0 0; border-radius: inherit; background: linear-gradient(90deg, rgba(255, 107, 74, 0.18), rgba(255, 189, 121, 0.28)); pointer-events: none; }
.age-gate-track-label { position: absolute; inset: 0; display: grid; place-items: center; color: #989ca3; font-size: 12px; font-weight: 650; pointer-events: none; }
.age-gate-thumb {
  position: absolute;
  top: 3px;
  left: 3px;
  z-index: 1;
  display: grid;
  width: 52px;
  height: 52px;
  place-items: center;
  border: 2px solid rgba(255, 255, 255, 0.9);
  border-radius: 50%;
  color: #fff;
  background: linear-gradient(145deg, #ff8765, #ff6246);
  box-shadow: 0 5px 14px rgba(255, 107, 74, 0.34);
  cursor: grab;
  touch-action: none;
}
.age-gate-thumb:active { cursor: grabbing; }
.age-gate-thumb:focus-visible, .age-gate-leave:focus-visible { outline: 3px solid #24262b; outline-offset: 3px; }
.age-gate-keyboard-hint { margin: 8px 0 0; color: #8b9098; font-size: 11px; }
.age-gate-underage { margin: 24px 0 0; color: #858a92; font-size: 12px; }
.age-gate-leave { margin-top: 7px; padding: 6px 10px; border: 0; border-radius: 8px; color: #666c74; background: transparent; font-size: 13px; font-weight: 700; cursor: pointer; }
.age-gate-success { display: grid; width: 58px; height: 58px; margin-bottom: 20px; place-items: center; border-radius: 50%; color: #fff; background: linear-gradient(145deg, #45c98a, #21a96b); box-shadow: 0 8px 24px rgba(33, 169, 107, 0.24); font-size: 23px; }
.age-gate-countdown h1 { max-width: 320px; }
.age-gate-countdown-ring { position: relative; width: 112px; height: 112px; margin: 24px 0 8px; }
.age-gate-countdown-ring svg { width: 100%; height: 100%; transform: rotate(-90deg); }
.age-gate-countdown-ring circle { fill: none; stroke-width: 7px; }
.age-gate-ring-track { stroke: #eceef1; }
.age-gate-ring-progress { stroke: #ff704e; stroke-linecap: round; stroke-dasharray: 232.5; transition: stroke-dashoffset 180ms linear; }
.age-gate-countdown-ring span { position: absolute; inset: 0; display: grid; place-items: center; font-size: 36px; font-weight: 800; font-variant-numeric: tabular-nums; }
.age-gate-countdown-label { margin: 0; color: #777d85; font-size: 14px; font-weight: 650; }
@media (max-width: 380px) {
  .age-gate { padding-right: 16px; padding-left: 16px; }
  .age-gate-card { padding: 30px 21px 24px; border-radius: 24px; }
}
@media (prefers-reduced-motion: reduce) {
  .age-gate-ring-progress { transition: none; }
}
</style>
