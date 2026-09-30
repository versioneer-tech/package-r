import { fetchURL, removePrefix, createURL } from "./utils";

export async function fetch(url: string, password: string = "") {
  const browserURL = removePrefix(url.split("?")[0]);
  url = removePrefix(url);

  const res = await fetchURL(
    `/api/public/share${url}`,
    {
      headers: { "X-SHARE-PASSWORD": encodeURIComponent(password) },
    },
    false
  );

  const data = (await res.json()) as Resource;
  data.url = `/share${browserURL}`;

  if (data.isDir) {
    if (!data.url.endsWith("/")) data.url += "/";
    data.items = data.items.map((item: any, index: any) => {
      item.index = index;
      item.url = `${data.url}${encodeURIComponent(item.name)}`;

      if (item.isDir) {
        item.url += "/";
      }

      return item;
    });
  }

  return data;
}

async function shareAction(
  url: string,
  method: ApiMethod,
  password: string = "",
  content?: any
) {
  url = removePrefix(url);

  const opts: ApiOpts = {
    method,
    headers: { "X-SHARE-PASSWORD": encodeURIComponent(password) },
  };

  if (content) {
    opts.body = content;
  }

  const res = await fetchURL(`/api/public/share${url}`, opts, false);

  return res;
}

export async function checksum(
  url: string,
  algo: ChecksumAlg | string,
  password: string = ""
) {
  const data = await shareAction(`${url}?checksum=${algo}`, "GET", password);
  return (await data.json()).checksums[algo];
}

export async function presign(url: string, password: string = "") {
  const data = await shareAction(`${url}?presign=true`, "GET", password);
  return (await data.json()).presignedURL;
}

export async function stacBrowserURL(url: string, password: string = "") {
  const data = await shareAction(`${url}?preview=true`, "GET", password);
  return (await data.json()).stacBrowserURL;
}

export function getOpenURL(res: Resource) {
  const params = {
    presign: "true",
    follow: "true",
    ...(res.token && { token: res.token }),
  };

  return createURL("api/public/share/" + res.hash + res.path, params, false);
}
