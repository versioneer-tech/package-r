import { defineStore } from "pinia";
import { detectLocale, setLocale } from "@/i18n";
import { cloneDeep } from "lodash-es";

export const useAuthStore = defineStore("auth", {
  state: (): {
    user: IUser | null;
    jwt: string;
  } => ({
    user: null,
    jwt: "",
  }),
  getters: {
    isLoggedIn: (state) => state.user !== null,
  },
  actions: {
    setUser(user: IUser) {
      if (user === null) {
        this.user = null;
        return;
      }

      setLocale(user.locale || detectLocale());
      this.user = user;
    },
    updateUser(user: Partial<IUser>) {
      if (user.locale) {
        setLocale(user.locale);
      }

      this.user = { ...this.user, ...cloneDeep(user) } as IUser;
    },
    clearUser() {
      this.$reset();
    },
  },
});
