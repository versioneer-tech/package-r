import { BaseClient, BaseResponse } from "geotiff";

interface ByteRange {
  start: number;
  end: number;
}

export class PreviewRequestError extends Error {
  constructor(public readonly status: number) {
    super(`Preview request failed with HTTP ${status}`);
    this.name = "PreviewRequestError";
  }
}

function parseRange(
  value: string | null,
  objectSize: number
): ByteRange | null {
  const match = /^bytes=(\d+)-(\d+)$/.exec(value ?? "");
  if (!match) return null;

  const start = Number(match[1]);
  const requestedEnd = Number(match[2]);
  if (
    !Number.isSafeInteger(start) ||
    !Number.isSafeInteger(requestedEnd) ||
    start < 0 ||
    requestedEnd < start ||
    start >= objectSize
  ) {
    return null;
  }

  return { start, end: Math.min(requestedEnd, objectSize - 1) };
}

class PartialContentResponse extends BaseResponse {
  private readonly compatiblePartialResponse: boolean;
  private data: Promise<ArrayBuffer> | null = null;

  constructor(
    private readonly response: Response,
    private readonly range: ByteRange | null,
    private readonly objectSize: number,
    private readonly onData: (bytes: number) => void
  ) {
    super();

    const contentLength = Number(response.headers.get("content-length"));
    const expectedLength = range ? range.end - range.start + 1 : 0;
    const expectedContentRange = range
      ? `bytes ${range.start}-${range.end}/${objectSize}`
      : "";
    const contentRange = response.headers.get("content-range");
    this.compatiblePartialResponse =
      [200, 206].includes(response.status) &&
      range !== null &&
      objectSize > expectedLength &&
      contentLength === expectedLength &&
      (contentRange === null || contentRange === expectedContentRange);
  }

  get status() {
    return this.compatiblePartialResponse ? 206 : this.response.status;
  }

  getHeader(name: string) {
    const value = this.response.headers.get(name);
    if (
      value === null &&
      name.toLowerCase() === "content-range" &&
      this.compatiblePartialResponse &&
      this.range
    ) {
      return `bytes ${this.range.start}-${this.range.end}/${this.objectSize}`;
    }

    return value ?? undefined;
  }

  async getData() {
    if (this.data === null) {
      this.data = this.response.arrayBuffer().then((data) => {
        this.onData(data.byteLength);
        return data;
      });
    }

    return this.data;
  }
}

export class PartialContentClient extends BaseClient {
  constructor(
    url: string,
    private readonly objectSize: number,
    private readonly onData: (bytes: number) => void = () => undefined
  ) {
    super(url);
  }

  async request(options: RequestInit = {}) {
    const headers = new Headers(options.headers);
    const range = parseRange(headers.get("range"), this.objectSize);
    const response = await fetch(this.url, {
      ...options,
      headers,
      cache: "no-store",
    });

    if (response.status === 401 || response.status === 403) {
      throw new PreviewRequestError(response.status);
    }

    return new PartialContentResponse(
      response,
      range,
      this.objectSize,
      this.onData
    );
  }
}
