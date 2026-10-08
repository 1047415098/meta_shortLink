import { request } from "./http";
// Log filters are sent in JSON so they never become part of the request URL.
export const getLogs = (filters) =>
  request("/request-logs/query", {
    method: "POST",
    body: JSON.stringify(filters),
  });
export const getLog = (id) =>
  request("/request-logs/" + encodeURIComponent(id));
