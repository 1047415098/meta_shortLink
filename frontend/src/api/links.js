import { request } from "./http";
export const listLinks = () => request("/links");
export const saveLink = (id, data) =>
  request("/links" + (id ? "/" + id : ""), {
    method: id ? "PATCH" : "POST",
    body: JSON.stringify(data),
  });
export const updateLink = (id, data) =>
  request("/links/" + id, { method: "PATCH", body: JSON.stringify(data) });
// Keep single-row and multi-row deletion on one atomic backend contract.
export const deleteLinks = (ids) =>
  request("/links/batch-delete", {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
