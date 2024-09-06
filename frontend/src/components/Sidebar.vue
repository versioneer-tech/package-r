<template>
  <div v-show="active" @click="closeHovers" class="overlay"></div>
  <nav :class="{ active }">
    <template v-if="isLoggedIn">
      <button
        class="action"
        @click="() => toRoot('')"
        :aria-label="Sources"
        :title="Sources"
      >
        <i class="material-icons">folder</i>
        <span>Sources</span>
      </button>
      <div v-for="(sourceName, index) in info.sourceNames" :key="index">
        <button
          class="action sub-action"
          @click="() => toRoot(sourceName)"
          :aria-label=sourceName
          :title=sourceName
          :class="{ active: $route.query.sourceName === sourceName }"
        >
          <span>{{ sourceName }}</span>
        </button>
      </div>

      <div v-if="user.perm.create">
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

    <!-- <div
      class="credits"
      v-if="isFiles && !disableUsedPercentage"
      style="width: 90%; margin: 2em 2.5em 3em 2.5em"
    >
      <progress-bar :val="info.usedPercentage" size="small"></progress-bar>
      <br />
      {{ info.used }} of {{ info.total }} used
    </div> -->

    <p class="credits">
      powered by 
      <span>
        <span v-if="disableExternal">packageR</span>
        <a
          v-else
          rel="noopener noreferrer"
          target="_blank"
          href="https://github.com/versioneer-tech/package-r"
          >packageR </a
        >
        <span> {{ version }}</span>
      </span>
      <!-- <span>
        <a @click="help">{{ $t("sidebar.help") }}</a>
      </span> -->
    </p>
  </nav>
</template>

<script>
import { reactive } from "vue";
import { mapActions, mapState } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import * as auth from "@/utils/auth";
import {
  name,
  version,
  signup,
  disableExternal,
  disableUsedPercentage,
  noAuth,
  loginPage,
} from "@/utils/constants";
import { files as api } from "@/api";
//import ProgressBar from "@/components/ProgressBar.vue";
import prettyBytes from "pretty-bytes";

const INFO_DEFAULT = { used: "0 B", total: "0 B", usedPercentage: 0, sourceNames: []};

export default {
  name: "sidebar",
  setup() {
    const info = reactive(INFO_DEFAULT);
    return { info };
  },
  components: {
//    ProgressBar,
  },
  inject: ["$showError"],
  computed: {
    ...mapState(useAuthStore, ["user", "isLoggedIn"]),
    ...mapState(useFileStore, ["isFiles", "reload"]),
    ...mapState(useLayoutStore, ["currentPromptName"]),
    active() {
      return this.currentPromptName === "sidebar";
    },
    signup: () => signup,
    name: () => name,
    version: () => version,
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && loginPage,
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers", "showHover"]),
    async fetchInfo() {
      let path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      try {
        let infoResponse = await api.info(path);
        return Object.assign(this.info, {
          used: prettyBytes(infoResponse.used, { binary: true }),
          total: prettyBytes(infoResponse.total, { binary: true }),
          usedPercentage: Math.round((infoResponse.used / infoResponse.total) * 100),
          sourceNames: infoResponse.sourceNames,
        });
      } catch (error) {
        this.$showError(error);
      }
    },
    toRoot(sourceName) {
      if (sourceName)
        this.$router.push({ path: "/files", query: { sourceName: sourceName }});
      else
        this.$router.push({ path: "/files"});
      this.closeHovers();
    },
    toSettings() {
      this.$router.push({ path: "/settings" });
      this.closeHovers();
    },
    // help() {
    //   this.showHover("help");
    // },
    logout: auth.logout,
  },
  watch: {
    isFiles(newValue) {
      newValue && this.fetchInfo();
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