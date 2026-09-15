import { request } from "./http";
import { buildQuery } from "../utils";
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
export const listMetaEvents = (filters) =>
  request("/meta/events?" + buildQuery(filters));
export const retryMetaEvent = (id) => post(`/events/${id}/retry`);
export const listPixels = (connection_id) =>
  request("/meta/pixels?" + buildQuery({ connection_id }));
export const savePixel = (id, data) =>
  request("/meta/pixels" + (id ? "/" + id : ""), {
    method: id ? "PATCH" : "POST",
    body: JSON.stringify(data),
  });
export const sendPixelTestEvent = (id, data) =>
  post(`/pixels/${id}/test-event`, data);
export const listCredentials = () => request("/meta/credentials");
export const listMetaAudit = () => request("/meta/audit");
export const rewrapCredentials = () => post("/credentials/rewrap");
export const inspectMetaSource = (url) => post("/source/inspect", { url });
