import { defineStore } from "pinia";

export const useLayoutStore = defineStore("layout", {
  state: (): {
    loading: boolean;
    prompts: PopupProps[];
    showShell: boolean | null;
  } => ({
    loading: false,
    prompts: [],
    showShell: false,
  }),
  getters: {
    currentPrompt(state) {
      return state.prompts.length > 0
        ? state.prompts[state.prompts.length - 1]
        : null;
    },
    currentPromptName(): string | null | undefined {
      return this.currentPrompt?.prompt;
    },
  },
  actions: {
    toggleShell() {
      this.showShell = !this.showShell;
    },
    setCloseOnCurrentPrompt(closeFunction: () => Promise<string>) {
      const prompt = this.prompts[this.prompts.length - 1];
      if (prompt) {
        prompt.close = closeFunction;
      }
    },
    showHover(value: PopupProps | string) {
      if (typeof value !== "object") {
        this.prompts.push({
          prompt: value,
          confirm: null,
          action: undefined,
          props: null,
          close: null,
        });
        return;
      }

      this.prompts.push({
        prompt: value.prompt,
        confirm: value?.confirm,
        action: value?.action,
        props: value?.props,
        close: value?.close,
      });
    },
    closeHovers() {
      this.prompts.pop()?.close?.();
    },
    clearLayout() {
      this.$reset();
    },
  },
});
