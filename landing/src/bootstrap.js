// Server-rendered data belongs to this single document request. Never fetch /:code again.
let bootstrap;
try {
  bootstrap = JSON.parse(
    document.getElementById("landing-data")?.textContent || "null",
  );
} catch {
  bootstrap = null;
}
export const landingData =
  bootstrap && typeof bootstrap === "object"
    ? bootstrap
    : { error: "This link is unavailable." };
