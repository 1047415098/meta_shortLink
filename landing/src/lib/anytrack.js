// AnyTrack is a comparison signal for deliberate WhatsApp clicks. Automatic
// redirects stay in the first-party report and never become this event.
export function trackAnyTrackManualContact(
  trigger,
  anyTrack = globalThis.AnyTrack,
) {
  if (trigger !== "manual" || typeof anyTrack !== "function") return false;
  try {
    anyTrack("trigger", "WhatsAppChatClick", {
      label: "manual_whatsapp_contact",
    });
    return true;
  } catch {
    // Third-party tracking must never prevent the visitor from opening WhatsApp.
    return false;
  }
}
