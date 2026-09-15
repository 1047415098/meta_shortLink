// Keep the browser-to-server contract explicit so arbitrary URL parameters never become HTTP headers.
const metaHeaderFields = [
  {
    names: ["fbclid", "fbcli"],
    header: "X-Meta-Fbclid",
    limit: 2048,
    encodedLimit: 2048,
  },
  {
    names: ["utm_source"],
    header: "X-Meta-Utm-Source",
    limit: 512,
    encodedLimit: 768,
  },
  {
    names: ["utm_campaign"],
    header: "X-Meta-Utm-Campaign",
    limit: 512,
    encodedLimit: 768,
  },
  {
    names: ["utm_content"],
    header: "X-Meta-Utm-Content",
    limit: 512,
    encodedLimit: 768,
  },
  {
    names: ["adset_name"],
    header: "X-Meta-Adset-Name",
    limit: 512,
    encodedLimit: 768,
  },
  {
    names: ["adset_id"],
    header: "X-Meta-Adset-Id",
    limit: 512,
    encodedLimit: 768,
  },
  {
    names: ["ad_id"],
    header: "X-Meta-Ad-Id",
    limit: 512,
    encodedLimit: 768,
  },
  {
    names: ["ad_name"],
    header: "X-Meta-Ad-Name",
    limit: 512,
    encodedLimit: 768,
  },
  {
    names: ["placement"],
    header: "X-Meta-Placement",
    limit: 512,
    encodedLimit: 768,
  },
  {
    names: ["site_source_name"],
    header: "X-Meta-Site-Source-Name",
    limit: 512,
    encodedLimit: 768,
  },
];

// Encode whole Unicode code points until the per-field byte budget is reached.
function headerValue(value, limit, encodedLimit) {
  const bounded = Array.from(String(value).trim()).slice(0, limit).join("");
  if (!bounded || bounded.includes("{{") || bounded.includes("}}")) return "";
  let encoded = "";
  for (const character of bounded) {
    const part = encodeURIComponent(character);
    if (encoded.length + part.length > encodedLimit) break;
    encoded += part;
  }
  return encoded;
}

// Build headers from the current entry URL; fbclid wins over the tolerated fbcli typo.
export function buildMetaAttributionHeaders(search = window.location.search) {
  const query = new URLSearchParams(search);
  const headers = {};
  for (const field of metaHeaderFields) {
    const value = field.names
      .map((name) => query.get(name))
      .find((candidate) => candidate !== null && candidate.trim() !== "");
    const encoded = headerValue(value || "", field.limit, field.encodedLimit);
    if (encoded) headers[field.header] = encoded;
  }
  return headers;
}
