import { createURL, fetchJSON } from "./utils";

export function getShares() {
  return fetchJSON<ConfiguredShare[]>("/api/shares");
}

export function getShareURL(share: ConfiguredShare) {
  return createURL(share.url.replace(/^\//, ""), {}, false);
}
