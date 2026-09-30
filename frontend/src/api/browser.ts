import { useBrowserStore } from "@/stores/browser";
import * as files from "./files";
import * as pub from "./pub";

export function fetch(url: string) {
  const browserStore = useBrowserStore();
  if (browserStore.source === "share") {
    return pub.fetch(url, browserStore.sharePassword);
  }
  return files.fetch(url);
}

export function checksum(url: string, algorithm: ChecksumAlg | string) {
  const browserStore = useBrowserStore();
  if (browserStore.source === "share") {
    return pub.checksum(url, algorithm, browserStore.sharePassword);
  }
  return files.checksum(url, algorithm);
}

export function presign(url: string) {
  const browserStore = useBrowserStore();
  if (browserStore.source === "share") {
    return pub.presign(url, browserStore.sharePassword);
  }
  return files.presign(url);
}

export function stacBrowserURL(url: string) {
  const browserStore = useBrowserStore();
  return pub.stacBrowserURL(url, browserStore.sharePassword);
}

export const getDownloadURL = files.getDownloadURL;
export const getPreviewURL = files.getPreviewURL;
