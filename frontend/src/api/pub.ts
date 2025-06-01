import { fetchURL, removePrefix, createURL } from "./utils";
import { baseURL } from "@/utils/constants";

export async function fetch(url: string, password: string = "") {
  url = removePrefix(url);

  const res = await fetchURL(
    `/api/public/share${url}`,
    {
      headers: { "X-SHARE-PASSWORD": encodeURIComponent(password) },
    },
    false
  );

  const data = (await res.json()) as Resource;
  data.url = `/share${url}`;

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

export function download(
  format: DownloadFormat,
  hash: string,
  token: string,
  ...files: string[]
) {
  let url = `${baseURL}/api/public/dl/${hash}`;

  if (files.length === 1) {
    url += encodeURIComponent(files[0]) + "?";
  } else {
    let arg = "";

    for (const file of files) {
      arg += encodeURIComponent(file) + ",";
    }

    arg = arg.substring(0, arg.length - 1);
    arg = encodeURIComponent(arg);
    url += `/?files=${arg}&`;
  }

  if (format) {
    url += `algo=${format}&`;
  }

  if (token) {
    url += `token=${token}&`;
  }

  window.open(url);
}

async function shareAction(url: string, method: ApiMethod, content?: any) {
  url = removePrefix(url);

  const opts: ApiOpts = {
    method,
  };

  if (content) {
    opts.body = content;
  }

  const res = await fetchURL(`/api/public/share${url}`, opts);

  return res;
}

export async function checksum(url: string, algo: ChecksumAlg | string) {
  const data = await shareAction(`${url}?checksum=${algo}`, "GET");
  return (await data.json()).checksums[algo];
}

export async function presign(url: string) {
  const data = await shareAction(`${url}?presign=true`, "GET");
  return (await data.json()).presignedURL;
}

export async function preview(url: string) {
  const data = await shareAction(`${url}?preview=true`, "GET");
  return (await data.json()).previewURL;
}

export function getDownloadURL(res: Resource, inline = false) {
  const params = {
    ...(inline && { inline: "true" }),
    ...(res.token && { token: res.token }),
  };

  return createURL("api/public/dl/" + res.hash + res.path, params, false);
}
