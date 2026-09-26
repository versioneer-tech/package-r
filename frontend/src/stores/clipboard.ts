import { defineStore } from "pinia";

export const useClipboardStore = defineStore("clipboard", {
  state: (): {
    key: string;
    items: ClipItem[];
    path?: string;
  } => ({
    key: "",
    items: [],
    path: undefined,
  }),
  getters: {},
  actions: {
    resetClipboard() {
      this.$reset();
    },
  },
});
