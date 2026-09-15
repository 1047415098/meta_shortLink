import { request } from "./http";
export const getSession = () => request("/auth/me");
export const signIn = (body) =>
  request("/auth/login", { method: "POST", body: JSON.stringify(body) });
export const signOut = () => request("/auth/logout", { method: "POST" });
