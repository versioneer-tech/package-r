type ApiMethod = "GET" | "POST" | "PUT" | "DELETE" | "PATCH";

type ApiContent =
  Blob | File | Pick<ReadableStreamDefaultReader<any>, "read"> | "";

interface ApiOpts {
  method?: ApiMethod;
  headers?: object;
  body?: any;
}

interface TusSettings {
  retryCount: number;
  chunkSize: number;
}

type ChecksumAlg = "md5" | "sha1" | "sha256" | "sha512";

interface Share {
  hash: string;
  url: string;
  expire?: any;
  description?: string;
}

interface ConfiguredShare {
  hash: string;
  path: string;
  url: string;
  expire: number;
  description?: string;
}

interface SearchParams {
  [key: string]: string;
}
