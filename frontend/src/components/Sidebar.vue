<template>
  <div v-show="active" @click="closeHovers" class="overlay"></div>
  <nav :class="{ active }">
    <template v-if="isLoggedIn">
      <button
        class="action"
        @click="() => toRoot('')"
        :aria-label="info.sources"
        :title="info.sources"
      >
        <i class="material-icons">folder</i>
        <span>{{ $t("sidebar.sources") }}</span>
      </button>

      <!--
      <div v-for="(source, index) in info.sources" :key="index">
        <button
          class="action sub-action"
          @click="() => toRoot(source.name)"
          :aria-label="source.name"
          :title="source.name"
          :class="{ active: $route.query.sourceName === source.name, fileset: source.subPath }"
        >
          <span>{{ source.friendlyName || source.name }}</span>
        </button>
      </div>
      -->
      <q-tree
          class="tree"
          :nodes="info.sources"
          node-key="name"
          children-key="sets"
          v-model:selected="selected"
          no-connectors
      >
        <!-- Custom slot for rendering node label -->
        <template v-slot:default-header="props">
          <div @click="selectSource(props.node)">
            {{ props.node.friendlyName || props.node.name  }}
          </div>
        </template>
      </q-tree>

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
          >packageR
        </a>
        <span> {{ version }}</span>
      </span>
      <!-- <span>
        <a @click="help">{{ $t("sidebar.help") }}</a>
      </span> -->
    </p>
  </nav>
</template>

<script lang="ts">
import {reactive, ref} from "vue";
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
import { Source } from "@/types/types";

import {QTree} from "quasar";

const INFO_DEFAULT = {
  used: "0 B",
  total: "0 B",
  usedPercentage: 0,
  sources: [] as Source[],
};

function groupSources(sources: Source[]): Source[] {
  // Create a map to hold parent sources by their secretName
  const sourceMap: Map<string, Source> = new Map();

  // First loop: add all parent sources (those without subPath) to the map
  sources.forEach((source) => {
    const name = source.secretName || source.name
    const parentSource = sourceMap.get(name);
    // take the first element with the name as parent element
    if (!parentSource) {
      source.sets = []
      sourceMap.set(source.secretName, source);
    } else {
      parentSource.sets.push(source);
    }
  });

  // Return the array of parent sources (with their sets populated)
  return Array.from(sourceMap.values());
}

export default {
  name: "sidebar",
  setup() {
    const info = reactive(INFO_DEFAULT);
    const selected = ref(null);
    return {
      info,
      selected
    };
  },
  components: {
    //    ProgressBar,
    QTree
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
          usedPercentage: Math.round(
            (infoResponse.used / infoResponse.total) * 100
          ),
          sources: groupSources(infoResponse.sources),
        });
      } catch (error) {
        this.$showError(error);
      }
    },
    label(source) {
      return source.friendlyName || source.name;
    },
    selectSource(node) {
      // console.log('clicked ' + node)
      if (node) {
        this.toRoot(node.name)
      }
    },
    toRoot(sourceName) {
      if (sourceName)
        this.$router.push({
          path: "/files",
          query: { sourceName: sourceName },
        });
      else this.$router.push({ path: "/files" });
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
.xtree {
  color: var(--action);
  margin-left: 10px;
  font-size: 0.8em;
}
.fileset {
  color: blue;
  padding-left: 20px;
}
</style>
