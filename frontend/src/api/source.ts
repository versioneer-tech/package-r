//import { fetchURL, fetchJSON } from "./utils";
import { NewSource } from "@/types/sources.js";

/*
export function get() {
  return fetchJSON<ISettings>(`/api/settings`, {});
}

 */

export async function update(source: NewSource): Promise<void> {
  alert("TODO implement API..." + source);
  /*
  await fetchURL(`/api/sources`, {
    method: "PUT",
    body: JSON.stringify(settings),
  });
     */
}
