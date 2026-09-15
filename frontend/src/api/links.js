import { request } from "./http";
export const listLinks = () => request("/links");
export const saveLink = (id, data) =>
  request("/links" + (id ? "/" + id : ""), {
    method: id ? "PATCH" : "POST",
    body: JSON.stringify(data),
  });
export const updateLink = (id, data) =>
  request("/links/" + id, { method: "PATCH", body: JSON.stringify(data) });
