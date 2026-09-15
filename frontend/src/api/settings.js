import { request } from "./http";
export const getSettings = () => request("/settings");
