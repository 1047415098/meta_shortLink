export async function reportLandingView({
  code,
  ticket,
  request = fetch,
  state = document.documentElement.dataset,
  attributionHeaders = {},
}) {
  if (!ticket || state.metaViewSent === "true") return;
  state.metaViewSent = "true";
  try {
    await request(`/${encodeURIComponent(code)}/view`, {
      method: "POST",
      // Mirror the trusted entry parameters for request-log correlation without changing attribution.
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        ...attributionHeaders,
      },
      body: new URLSearchParams({ ticket }).toString(),
      keepalive: true,
    });
  } catch {
    // Measurement must never interrupt the visitor flow.
  }
}
