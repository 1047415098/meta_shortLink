import { request } from "./http.js";
import { buildQuery } from "../utils/index.js";
import {
  tiktokConnectionPayload,
  tiktokPixelPayload,
} from "../utils/tiktok.js";

export const listTikTokConnections = () => request("/tiktok-connections");

export const saveTikTokConnection = (id, data) =>
  request(`/tiktok-connections${id ? `/${id}` : ""}`, {
    method: id ? "PATCH" : "POST",
    // 编辑时不提交空 Token，避免覆盖服务端已经加密保存的凭证。
    body: JSON.stringify(tiktokConnectionPayload(data, Boolean(id))),
  });

export const deleteTikTokConnection = (id) =>
  request(`/tiktok-connections/${id}`, { method: "DELETE" });

export const listTikTokPixels = (connection_id) =>
  request(`/tiktok-pixels?${buildQuery({ connection_id })}`);

export const saveTikTokPixel = (id, data) =>
  request(`/tiktok-pixels${id ? `/${id}` : ""}`, {
    method: id ? "PATCH" : "POST",
    body: JSON.stringify(tiktokPixelPayload(data)),
  });

export const deleteTikTokPixel = (id) =>
  request(`/tiktok-pixels/${id}`, { method: "DELETE" });

export const testTikTokPixel = (id) =>
  request(`/tiktok-pixels/${id}/test`, { method: "POST" });

export const listTikTokEvents = (filters = {}) =>
  request(`/tiktok-events?${buildQuery(filters)}`);

export const retryTikTokEvent = (id) =>
  request(`/tiktok-events/${encodeURIComponent(id)}/retry`, {
    method: "POST",
  });
