export async function submitAudioNovelContact({ code, ticket, request = fetch }) {
  const body = new URLSearchParams({ ticket, trigger: "manual" });
  const response = await request(`/audio-novel/${encodeURIComponent(code)}/contact`, {
    method: "POST",
    headers: { Accept: "application/json", "Content-Type": "application/x-www-form-urlencoded" },
    body
  });
  if (!response.ok) throw new Error("The passage to WhatsApp is temporarily unavailable.");
  return response.json();
}
