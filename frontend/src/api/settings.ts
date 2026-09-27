import { createURL, fetchJSON, fetchURL } from "./utils";

export type ProfileUpdate = Partial<
  Pick<
    IUser,
    | "locale"
    | "viewMode"
    | "singleClick"
    | "sorting"
    | "hideDotfiles"
    | "dateFormat"
  >
>;

export async function updateProfile(profile: ProfileUpdate) {
  await fetchURL("/api/profile", {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(profile),
  });
}

export function getShares() {
  return fetchJSON<ConfiguredShare[]>("/api/shares");
}

export function getShareURL(share: ConfiguredShare) {
  return createURL(share.url.replace(/^\//, ""), {}, false);
}
