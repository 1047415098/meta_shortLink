import { request } from "./http";
import { buildQuery } from "../utils";
export const getAnalytics = (filters) =>
  request("/analytics?" + buildQuery(filters));
export const getVisits = (filters) => request("/clicks?" + buildQuery(filters));
// Submit filters in JSON while retaining the shared session and request headers.
export const getLinkStats = (id, filters) =>
  request("/links/" + encodeURIComponent(id) + "/stats", {
    method: "POST",
    body: JSON.stringify(filters),
  });
export async function exportVisits(filters) {
  const response = await fetch(
    "/api/v1/exports/clicks?" + buildQuery(filters),
    { credentials: "same-origin" },
  );
  if (!response.ok) throw new Error("导出失败，请检查登录状态或缩小日期范围");
  return response.blob();
}
