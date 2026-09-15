import { request } from "./http";
export const getLogs = (query) => request("/request-logs?" + query);
export const getLog = (id) =>
  request("/request-logs/" + encodeURIComponent(id));
