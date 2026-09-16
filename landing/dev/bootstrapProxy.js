import { readFile } from "node:fs/promises";

const shortCodePattern = /^\/[a-zA-Z0-9_-]{3,40}\/?$/;
const reservedPaths = new Set([
  "/admin",
  "/api",
  "/assets",
  "/healthz",
  "/landing-assets",
]);

export function isLandingPagePath(rawURL = "") {
  const pathname = new URL(rawURL, "http://vite.local").pathname;
  if (!shortCodePattern.test(pathname)) return false;
  return !reservedPaths.has(pathname.replace(/\/$/, ""));
}

export function extractLandingBootstrap(page) {
  // The Go renderer owns the signed ticket; development only transplants that
  // exact JSON script into Vite's HMR-enabled document.
  const match = page.match(
    /<script\b[^>]*\bid=["']landing-data["'][^>]*>[\s\S]*?<\/script>/i,
  );
  if (!match) throw new Error("Landing bootstrap script is missing");
  return match[0];
}

function copyHeader(response, name, value) {
  if (value) response.setHeader(name, value);
}

export function landingBootstrapProxy({
  backendOrigin = "http://127.0.0.1:8080",
} = {}) {
  return {
    name: "linkscope-landing-bootstrap",
    configureServer(server) {
      server.middlewares.use(async (request, response, next) => {
        if (
          !["GET", "HEAD"].includes(request.method || "") ||
          !isLandingPagePath(request.url)
        ) {
          next();
          return;
        }

        try {
          const upstreamHeaders = {};
          // Preserve the browser identity and first-party visitor cookie so the
          // development request follows the same attribution path as port 8080.
          for (const name of [
            "cookie",
            "user-agent",
            "accept-language",
            "referer",
          ]) {
            if (request.headers[name])
              upstreamHeaders[name] = request.headers[name];
          }
          const upstream = await fetch(
            new URL(request.url || "/", backendOrigin),
            {
              method: request.method,
              headers: upstreamHeaders,
              redirect: "manual",
            },
          );
          const body = request.method === "HEAD" ? "" : await upstream.text();
          response.statusCode = upstream.status;
          copyHeader(response, "Cache-Control", "no-store");
          for (const cookie of upstream.headers.getSetCookie?.() || []) {
            response.appendHeader("Set-Cookie", cookie);
          }

          let bootstrap;
          try {
            bootstrap = extractLandingBootstrap(body);
          } catch {
            // Direct-mode and unavailable non-Vue responses remain byte-for-byte
            // backend pages while landing mode receives Vite HMR below.
            copyHeader(
              response,
              "Content-Type",
              upstream.headers.get("content-type") ||
                "text/html; charset=utf-8",
            );
            copyHeader(
              response,
              "Content-Security-Policy",
              upstream.headers.get("content-security-policy"),
            );
            response.end(body);
            return;
          }

          const source = await readFile(
            new URL("../index.html", import.meta.url),
            "utf8",
          );
          const document = source.replace(
            "<!--LANDING_BOOTSTRAP-->",
            bootstrap,
          );
          const transformed = await server.transformIndexHtml(
            request.url || "/",
            document,
          );
          copyHeader(response, "Content-Type", "text/html; charset=utf-8");
          response.end(request.method === "HEAD" ? "" : transformed);
        } catch (error) {
          server.config.logger.error(
            `Landing development proxy failed: ${error.message}`,
          );
          response.statusCode = 502;
          response.setHeader("Content-Type", "text/plain; charset=utf-8");
          response.end("The local Go service is unavailable on port 8080.");
        }
      });
    },
  };
}
