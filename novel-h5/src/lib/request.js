export function createRequestGate() {
  let latest = 0;
  return {
    // 每次新请求递增代次，较慢返回的旧语言响应不会覆盖当前页面。
    next() { latest += 1; return latest; },
    isCurrent(version) { return version === latest; },
    cancel() { latest += 1; },
  };
}
