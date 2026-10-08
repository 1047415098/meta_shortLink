import { request } from "./http";
export const getAnalytics = (filters) =>
  request("/analytics/query", {
    method: "POST",
    body: JSON.stringify(filters),
  });
export const getVisits = (filters) =>
  request("/clicks/query", { method: "POST", body: JSON.stringify(filters) });
// Submit filters in JSON while retaining the shared session and request headers.
export const getLinkStats = (id, filters) =>
  request("/links/" + encodeURIComponent(id) + "/stats", {
    method: "POST",
    body: JSON.stringify(filters),
  });
export async function exportVisits(filters) {
  // Export filters stay in the JSON body, not browser history or proxy URLs.
  const response = await fetch("/api/v1/exports/clicks", {
    method: "POST",
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      "X-Requested-With": "XMLHttpRequest",
    },
    body: JSON.stringify(filters),
  });
  if (!response.ok) throw new Error("导出失败，请检查登录状态或缩小日期范围");
  return response.blob();
}
