import { trackAnyTrackManualContact } from "./anytrack.js";

// Submit through Fetch so the consultation request and mirrored Meta headers are visible before navigation.
export async function submitLandingContact({
  code,
  ticket,
  trigger,
  attributionHeaders = {},
  request = fetch,
  anyTrack = globalThis.AnyTrack,
  navigate = (target) => window.location.assign(target),
}) {
  const response = await request(`/${encodeURIComponent(code)}/contact`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/x-www-form-urlencoded",
      "X-Requested-With": "XMLHttpRequest",
      ...attributionHeaders,
    },
    body: new URLSearchParams({ ticket, trigger }).toString(),
  });
  if (!response.ok)
    throw new Error(`Consultation request failed: ${response.status}`);
  const payload = await response.json();
  if (!payload?.target_url) throw new Error("Consultation target is missing");
  // Report only after the first-party endpoint confirms the manual action.
  trackAnyTrackManualContact(trigger, anyTrack);
  navigate(payload.target_url);
  return payload.target_url;
}

// The countdown owns manual/automatic classification and can delegate submission to Fetch.
export function createContactCountdown({
  form,
  trigger,
  delay,
  onRemaining,
  restored = false,
  events = window,
  now = Date.now,
  schedule = setInterval,
  cancel = clearInterval,
  onSubmit,
}) {
  let timer;
  let automaticSubmit = false;
  let submitted = false;
  const stopTimer = () => {
    if (timer !== undefined) cancel(timer);
    timer = undefined;
  };
  const handleSubmit = (event) => {
    // Fetch mode blocks duplicate navigation while the first signed request is in flight.
    if (submitted) {
      if (onSubmit) event.preventDefault();
      return;
    }
    trigger.value = automaticSubmit ? "auto" : "manual";
    submitted = true;
    stopTimer();
    if (onSubmit) {
      event.preventDefault();
      onSubmit(trigger.value);
    }
  };
  const handleHide = () => {
    stopTimer();
    onRemaining(null);
  };
  const handleShow = (event) => {
    if (!event.persisted) return;
    stopTimer();
    automaticSubmit = false;
    submitted = false;
    trigger.value = "manual";
    onRemaining(null);
  };
  form.addEventListener("submit", handleSubmit);
  events.addEventListener("pagehide", handleHide);
  events.addEventListener("pageshow", handleShow);
  trigger.value = "manual";
  const seconds = Number(delay);
  if (!restored && Number.isFinite(seconds) && seconds > 0) {
    const deadline = now() + seconds * 1000;
    onRemaining(Math.ceil(seconds));
    timer = schedule(() => {
      if (submitted) return;
      const remaining = Math.max(0, Math.ceil((deadline - now()) / 1000));
      onRemaining(remaining);
      if (remaining !== 0) return;
      stopTimer();
      automaticSubmit = true;
      try {
        form.requestSubmit();
      } finally {
        automaticSubmit = false;
      }
    }, 200);
  }
  return () => {
    stopTimer();
    form.removeEventListener("submit", handleSubmit);
    events.removeEventListener("pagehide", handleHide);
    events.removeEventListener("pageshow", handleShow);
  };
}
