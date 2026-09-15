import { request } from "./http";
export const uploadSpend = (body) =>
  request("/ad-spend/import", { method: "POST", body });
