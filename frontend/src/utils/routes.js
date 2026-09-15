export function safeReturnPath(value) {
  return typeof value === "string" &&
    value.startsWith("/admin/") &&
    !value.startsWith("/admin/login") &&
    !/[\\\r\n]/.test(value)
    ? value
    : "/admin/overview";
}
