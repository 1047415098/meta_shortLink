// Development keeps the legacy home entry; production receives an optional bound story slug.
const fallback = { link: import.meta.env?.DEV ? { code:"hello", entry_story_slug:"", time_spent_threshold:0 } : null, ticket:"", surface:"novel", cookie_enabled:false, meta_measurement:false, error:null };
export function readBootstrap(documentRef = document) {
  const node = documentRef.getElementById("novel-h5-data");
  if (!node?.textContent) return fallback;
  try { return { ...fallback, ...JSON.parse(node.textContent) }; }
  catch { return { ...fallback, error:{ status:503, message:"The story archive could not be opened." } }; }
}
