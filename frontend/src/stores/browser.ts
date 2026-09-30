import { defineStore } from "pinia";

type BrowserSource = "authenticated" | "share";

type BrowserPermissions = Pick<
  Permissions,
  "create" | "delete" | "download" | "execute" | "modify" | "rename"
>;

export const EMPTY_PERMISSIONS: Readonly<BrowserPermissions> = Object.freeze({
  create: false,
  delete: false,
  download: false,
  execute: false,
  modify: false,
  rename: false,
});

export const useBrowserStore = defineStore("browser", {
  state: (): {
    source: BrowserSource;
    basePath: string;
    sharePassword: string;
    viewMode: ViewModeType;
  } => ({
    source: "authenticated",
    basePath: "/files",
    sharePassword: "",
    viewMode: "list",
  }),
  getters: {
    readOnly: (state) => state.source === "share",
  },
  actions: {
    useAuthenticatedSource() {
      this.$reset();
    },
    useShareSource(hash: string, password: string) {
      this.source = "share";
      this.basePath = `/share/${hash}`;
      this.sharePassword = password;
    },
  },
});
