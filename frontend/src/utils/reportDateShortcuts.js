import { DEFAULT_REPORT_TIMEZONE } from "../constants/reportTimezones.js";

// Convert a report-day offset into a local noon Date so Element Plus keeps the
// intended YYYY-MM-DD value regardless of the browser's own timezone.
function dateForReportDay(timezone, offset = 0) {
  const today = new Date().toLocaleDateString("en-CA", {
    timeZone: timezone || DEFAULT_REPORT_TIMEZONE,
  });
  const date = new Date(`${today}T12:00:00`);
  date.setDate(date.getDate() + offset);
  return date;
}

// All operator reports expose exactly the same three calendar shortcuts.
export function reportDateShortcuts(
  getTimezone = () => DEFAULT_REPORT_TIMEZONE,
) {
  const range = (days) => {
    const timezone = getTimezone() || DEFAULT_REPORT_TIMEZONE;
    return [dateForReportDay(timezone, 1 - days), dateForReportDay(timezone)];
  };
  return [
    { text: "今天", value: () => range(1) },
    { text: "最近 7 天", value: () => range(7) },
    { text: "最近 30 天", value: () => range(30) },
  ];
}
