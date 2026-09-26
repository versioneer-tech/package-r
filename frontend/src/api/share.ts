import { createURL, fetchJSON, removePrefix } from "./utils";

export function list() {
  return fetchJSON<ConfiguredShare[]>("/api/shares");
}

export async function get(url: string) {
  return fetchJSON<Share[]>(`/api/share${removePrefix(url)}`);
}

export function getShareURL(share: Share) {
  return createURL(share.url.replace(/^\//, ""), {}, false);
}
