// IANA Etc/GMT names reverse their signs: GMT+8 is the fixed UTC-8 default.
export const DEFAULT_REPORT_TIMEZONE = "Etc/GMT+8";

// Keep all operator reporting views on the same two non-DST calendar boundaries.
export const REPORT_TIMEZONES = [
  { label: "UTC-8（固定）", value: "Etc/GMT+8" },
  { label: "UTC+8（固定）", value: "Etc/GMT-8" },
];
