type Source = { key: string; label: string; kind: "REST" | "WebSocket" | "Подписка"; state: string };
const pages = new Map<string, Map<string, Source>>();
const listeners = new Set<() => void>();
let version = 0;
export function subscribeSources(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; }
export function sourceVersion() { return version; }
export function pageSources(page: string) { return [...(pages.get(page)?.values() || [])]; }
export function recordSource(page: string, source: Source) {
  let entries = pages.get(page); if (!entries) { entries = new Map(); pages.set(page, entries); }
  const previous = entries.get(source.key);
  if (previous?.label === source.label && previous.state === source.state) return;
  entries.set(source.key, source); version++; listeners.forEach(listener => listener());
}
export function recordEndpoint(page: string, method: string, path: string, state: string, kind: Source["kind"] = "REST") {
  // Display the endpoint only; query parameters, tokens and request bodies stay private.
  const endpoint = path.split("?")[0];
  recordSource(page, { key: `${method} ${endpoint}`, label: `${method} ${endpoint}`, kind, state });
}
