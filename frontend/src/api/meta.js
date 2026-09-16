import { request } from "./http.js";
import { buildQuery } from "../utils/index.js";
const post = (path, data) =>
  request("/meta" + path, {
    method: "POST",
    ...(data ? { body: JSON.stringify(data) } : {}),
  });
export const listConnections = () => request("/meta/connections");
export const saveConnection = (id, data) =>
  request("/meta/connections" + (id ? "/" + id : ""), {
    method: id ? "PATCH" : "POST",
    body: JSON.stringify(data),
  });
// Deletion is allowed only after the backend confirms that no live binding or
// historical business record depends on the selected configuration.
export const deleteConnection = (id) =>
  request(`/meta/connections/${id}`, { method: "DELETE" });
export const listMetaEvents = (filters) =>
  request("/meta/events?" + buildQuery(filters));
export const retryMetaEvent = (id) => post(`/events/${id}/retry`);
export const listPixels = (connection_id) =>
  request("/meta/pixels?" + buildQuery({ connection_id }));
// Credential plaintext is requested only when an administrator opens one
// Pixel for editing; the collection endpoint remains secret-free.
export const getPixelCredential = (id) =>
  request(`/meta/pixels/${id}/credential`);
export const savePixel = (id, data) =>
  request("/meta/pixels" + (id ? "/" + id : ""), {
    method: id ? "PATCH" : "POST",
    body: JSON.stringify(data),
  });
export const deletePixel = (id) =>
  request(`/meta/pixels/${id}`, { method: "DELETE" });
export const sendPixelTestEvent = (id, data) =>
  post(`/pixels/${id}/test-event`, data);
export const listCredentials = () => request("/meta/credentials");
export const listMetaAudit = () => request("/meta/audit");
export const rewrapCredentials = () => post("/credentials/rewrap");
export const inspectMetaSource = (url) => post("/source/inspect", { url });
