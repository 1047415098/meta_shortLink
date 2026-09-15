let onUnauthorized = () => {};
export function setUnauthorizedHandler(handler) {
  onUnauthorized = handler;
}
export async function request(path, options = {}) {
  const headers = { "X-Requested-With": "XMLHttpRequest", ...options.headers };
  if (options.body && !(options.body instanceof FormData))
    headers["Content-Type"] = "application/json";
  const response = await fetch("/api/v1" + path, {
    credentials: "same-origin",
    ...options,
    headers,
  });
  if (!response.ok) {
    let message = "请求失败，请稍后重试";
    try {
      const data = await response.json();
      message = data.error || data.message || message;
    } catch {}
    const error = new Error(message);
    error.status = response.status;
    if (response.status === 401 && !path.startsWith("/auth/")) onUnauthorized();
    throw error;
  }
  if (response.status === 204) return null;
  return response.json();
}
