import { request } from "./http.js";
// 日志筛选统一发送数字状态码；清空输入时用 0 表示“不限状态”。
export const getLogs = (filters) => {
  const rawStatus = String(filters.status ?? "").trim();
  const status = rawStatus === "" ? 0 : Number(rawStatus);
  if (
    !Number.isInteger(status) ||
    (status !== 0 && (status < 100 || status > 599))
  )
    return Promise.reject(new Error("状态码请输入 100 至 599"));
  // 筛选条件保留在 JSON 请求体内，不写入浏览器地址栏。
  return request("/request-logs/query", {
    method: "POST",
    body: JSON.stringify({ ...filters, status }),
  });
};
export const getLog = (id) =>
  request("/request-logs/" + encodeURIComponent(id));
