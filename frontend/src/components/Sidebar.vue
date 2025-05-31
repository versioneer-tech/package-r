<template>
  <div v-show="active" @click="closeHovers" class="overlay"></div>
  <nav :class="{ active }">
    <template v-if="isLoggedIn">
      <button
        class="action"
        @click="toFiles('')"
        :aria-label="$t('sidebar.home')"
        :title="$t('sidebar.home')"
      >
        <i class="material-icons">folder</i>
        <span>{{ $t("sidebar.home") }}</span>
      </button>

      <div v-if="this.sources.length > 0">
        <button
          class="action"
          @click="toFiles('sources')"
          :aria-label="$t('sidebar.mySources')"
          :title="$t('sidebar.mySources')"
        >
          <i class="material-icons">folder</i>
          <span>{{ $t("sidebar.mySources") }}</span>
        </button>
      </div>

      <div v-if="this.packages.length > 0">
        <button
          class="action"
          @click="toFiles('packages')"
          :aria-label="$t('sidebar.myPackages')"
          :title="$t('sidebar.myPackages')"
        >
          <i class="material-icons">folder</i>
          <span>{{ $t("sidebar.myPackages") }}</span>
        </button>
      </div>

      <div v-if="user.perm.create && hasWritePermissions(req)">
        <button
          @click="showHover('newDir')"
          class="action"
          :aria-label="$t('sidebar.newFolder')"
          :title="$t('sidebar.newFolder')"
        >
          <i class="material-icons">create_new_folder</i>
          <span>{{ $t("sidebar.newFolder") }}</span>
        </button>

        <button
          @click="showHover('newFile')"
          class="action"
          :aria-label="$t('sidebar.newFile')"
          :title="$t('sidebar.newFile')"
        >
          <i class="material-icons">note_add</i>
          <span>{{ $t("sidebar.newFile") }}</span>
        </button>
      </div>

      <div>
        <button
          class="action"
          @click="toSettings"
          :aria-label="$t('sidebar.settings')"
          :title="$t('sidebar.settings')"
        >
          <i class="material-icons">settings_applications</i>
          <span>{{ $t("sidebar.settings") }}</span>
        </button>

        <button
          v-if="canLogout"
          @click="logout"
          class="action"
          id="logout"
          :aria-label="$t('sidebar.logout')"
          :title="$t('sidebar.logout')"
        >
          <i class="material-icons">exit_to_app</i>
          <span>{{ $t("sidebar.logout") }}</span>
        </button>
      </div>
    </template>

    <template v-else>
      <router-link
        class="action"
        to="/login"
        :aria-label="$t('sidebar.login')"
        :title="$t('sidebar.login')"
      >
        <i class="material-icons">exit_to_app</i>
        <span>{{ $t("sidebar.login") }}</span>
      </router-link>

      <router-link
        v-if="signup"
        class="action"
        to="/login"
        :aria-label="$t('sidebar.signup')"
        :title="$t('sidebar.signup')"
      >
        <i class="material-icons">person_add</i>
        <span>{{ $t("sidebar.signup") }}</span>
      </router-link>
    </template>

    <div
      class="credits"
      v-if="isFiles && !disableUsedPercentage"
      style="width: 90%; margin: 2em 2.5em 3em 2.5em"
    >
      <progress-bar :val="usage.usedPercentage" size="small"></progress-bar>
      <br />
      {{ usage.used }} of {{ usage.total }} used
    </div>

    <p class="credits">
      <span>
        <span v-if="disableExternal">packageR</span>
        <a
          v-else
          rel="noopener noreferrer"
          target="_blank"
          href="https://github.com/versioneer-tech/package-r"
        >
          packageR
        </a>
        <span> {{ " " }} {{ version }}</span>
      </span>
      <span>
        <a @click="help">{{ $t("sidebar.help") }}</a>
      </span>
    </p>
  </nav>
</template>

<script>
import { reactive } from "vue";
import { mapActions, mapState } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { baseURL } from "@/utils/constants";

import * as auth from "@/utils/auth";
import {
  version,
  signup,
  disableExternal,
  disableUsedPercentage,
  noAuth,
  loginPage,
} from "@/utils/constants";
import { files as api } from "@/api";
import ProgressBar from "@/components/ProgressBar.vue";
import prettyBytes from "pretty-bytes";

const USAGE_DEFAULT = { used: "0 B", total: "0 B", usedPercentage: 0 };

export default {
  name: "sidebar",
  setup() {
    const usage = reactive(USAGE_DEFAULT);
    const sources = reactive([]);
    const packages = reactive([]);
    return { usage, sources, packages };
  },
  components: {
    ProgressBar,
  },
  inject: ["$showError"],
  computed: {
    ...mapState(useAuthStore, ["user", "isLoggedIn"]),
    ...mapState(useFileStore, ["isFiles", "reload", "req"]),
    ...mapState(useLayoutStore, ["currentPromptName"]),
    active() {
      return this.currentPromptName === "sidebar";
    },
    signup: () => signup,
    version: () => version,
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && loginPage,
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers", "showHover"]),
    async fetchUsage() {
      let path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      let usageStats = USAGE_DEFAULT;
      if (this.disableUsedPercentage) {
        return Object.assign(this.usage, usageStats);
      }
      try {
        let usage = await api.usage(path);
        usageStats = {
          used: prettyBytes(usage.used, { binary: true }),
          total: prettyBytes(usage.total, { binary: true }),
          usedPercentage: Math.round((usage.used / usage.total) * 100),
        };
      } catch (error) {
        this.$showError(error);
      }
      return Object.assign(this.usage, usageStats);
    },
    getSources() {
      const ssl = window.location.protocol === "https:";
      const protocol = ssl ? "wss:" : "ws:";
      const authStore = useAuthStore();
      const url = `${protocol}//${window.location.host}${baseURL}/api/command/?auth=${authStore.jwt}`;

      try {
        const websocket = new WebSocket(url);

        websocket.onopen = () => {
          console.log("WebSocket connection opened");
          websocket.send("establish-sources");
        };

        websocket.onmessage = (event) => {
          console.log("Message received from server:", event.data);
          if (event.data) {
            const newSource = { name: event.data };
            const existingIndex = this.sources.findIndex(
              (s) => s.name === newSource.name
            );
            if (existingIndex !== -1) {
              this.sources[existingIndex] = newSource;
            } else {
              this.sources.push(newSource);
            }
          }
        };

        websocket.onerror = (error) => {
          console.error("WebSocket error:", error);
          this.$showError("WebSocket error occurred");
        };

        websocket.onclose = () => {
          console.log("WebSocket connection closed");
        };
      } catch (error) {
        console.error("Error creating WebSocket:", error);
        this.$showError("An error occurred while creating WebSocket");
      }
    },
    getPackages() {
      const ssl = window.location.protocol === "https:";
      const protocol = ssl ? "wss:" : "ws:";
      const authStore = useAuthStore();
      const url = `${protocol}//${window.location.host}${baseURL}/api/command/?auth=${authStore.jwt}`;

      try {
        const websocket = new WebSocket(url);

        websocket.onopen = () => {
          console.log("WebSocket connection opened");
          websocket.send("establish-packages");
        };

        websocket.onmessage = (event) => {
          console.log("Message received from server:", event.data);
          if (event.data) {
            const newPackage = { name: event.data };
            const existingIndex = this.packages.findIndex(
              (p) => p.name === newPackage.name
            );
            if (existingIndex !== -1) {
              this.packages[existingIndex] = newPackage;
            } else {
              this.packages.push(newPackage);
            }
          }
        };

        websocket.onerror = (error) => {
          console.error("WebSocket error:", error);
          this.$showError("WebSocket error occurred"); // Show error message
        };

        websocket.onclose = () => {
          console.log("WebSocket connection closed");
        };
      } catch (error) {
        console.error("Error creating WebSocket:", error);
        this.$showError("An error occurred while creating WebSocket");
      }
    },
    toFiles(path) {
      this.$router.push({ path: `/files/${path}` });
      this.closeHovers();
    },
    toSettings() {
      this.$router.push({ path: "/settings" });
      this.closeHovers();
    },
    help() {
      this.showHover("help");
    },
    logout: auth.logout,
    hasWritePermissions(req) {
      if (req?.path.startsWith("/.sources/")) {
        return false;
      }
      if (req?.path.startsWith("/.packages/")) {
        return false;
      }
      const OWNER_WRITE = 0o200;
      let mode = req?.mode & 0o777;
      return (mode & OWNER_WRITE) !== 0;
    },
  },
  watch: {
    isFiles(newValue) {
      newValue && this.fetchUsage();
      newValue && this.user.perm.share && this.getSources() && this.getPackages()
    },
    req() {
      // Watch logic for req
    },
  },
};
</script>
<style scoped>
.sub-action {
  margin-left: 30px;
  font-size: 0.8em;
}
.sub-action.active {
  background-color: lightgray;
}
</style>
