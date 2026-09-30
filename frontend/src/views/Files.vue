<template>
  <div>
    <header-bar
      v-if="error || fileStore.req?.type === null"
      :show-menu="!browserStore.readOnly"
      showLogo
    />

    <breadcrumbs :base="browserStore.basePath" />
    <slot v-if="error" name="error" :error="error">
      <errors :errorCode="error.status" />
    </slot>

    <component
      v-else-if="currentView"
      :is="currentView"
      :url="fileStore.req?.presignedURL"
    />

    <div v-else-if="currentView !== null">
      <h2 class="message delayed">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ t("files.loading") }}</span>
      </h2>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  computed,
  defineAsyncComponent,
  onBeforeUnmount,
  onMounted,
  onUnmounted,
  inject,
  ref,
  watch,
} from "vue";
import { browser as api } from "@/api";
import { storeToRefs } from "pinia";
import { useBrowserStore } from "@/stores/browser";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { useUploadStore } from "@/stores/upload";

import HeaderBar from "@/components/header/HeaderBar.vue";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
import Errors from "@/views/Errors.vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";
import FileListing from "@/views/files/FileListing.vue";
import { StatusError } from "@/api/utils";
import { name } from "../utils/constants";

const Editor = defineAsyncComponent(() => import("@/views/files/Editor.vue"));
const Preview = defineAsyncComponent(() => import("@/views/files/Preview.vue"));
const TextViewer = defineAsyncComponent(
  () => import("@/views/files/TextViewer.vue")
);

const layoutStore = useLayoutStore();
const browserStore = useBrowserStore();
const fileStore = useFileStore();
const uploadStore = useUploadStore();

const props = withDefaults(
  defineProps<{
    source?: "authenticated" | "share";
    password?: string;
    requestKey?: number;
  }>(),
  {
    source: "authenticated",
    password: "",
    requestKey: 0,
  }
);

const { reload } = storeToRefs(fileStore);
const { error: uploadError } = storeToRefs(uploadStore);

const route = useRoute();
const { t } = useI18n({});
const $showError = inject<IToastError>("$showError")!;

const clean = (path: string) => {
  return path.endsWith("/") ? path.slice(0, -1) : path;
};

const error = ref<StatusError | null>(null);

const shareHash = () => {
  const path = route.params.path;
  if (Array.isArray(path)) return path[0] ?? "";
  return path ?? "";
};

const configureBrowser = () => {
  if (props.source === "share") {
    browserStore.useShareSource(shareHash(), props.password);
    return;
  }
  browserStore.useAuthenticatedSource();
};

configureBrowser();

const currentView = computed(() => {
  const req = fileStore.req;

  if (!req || req.type === undefined) {
    return null;
  }

  if (req.isDir) {
    return FileListing;
  }

  if (req.type === "text" || req.type === "textImmutable") {
    return browserStore.readOnly ? TextViewer : Editor;
  }

  if (
    req.type === "pdf" ||
    req.type === "image" ||
    req.type === "audio" ||
    req.type === "video" ||
    req.type === "tiff"
  ) {
    return Preview;
  }

  return null;
});

watch(currentView, (view) => {
  if (view === null && fileStore.req && !fileStore.req.isDir) {
    error.value = new StatusError("preview not allowed", 415);
  } else {
    error.value = null;
  }
});

onMounted(() => {
  fetchData();
  fileStore.isFiles = true;
  window.addEventListener("keydown", keyEvent);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", keyEvent);
});

onUnmounted(() => {
  fileStore.isFiles = false;
  if (layoutStore.showShell) {
    layoutStore.toggleShell();
  }
  fileStore.updateRequest(null);
  if (props.source === "share") {
    browserStore.useAuthenticatedSource();
  }
});

watch(route, (to, from) => {
  configureBrowser();
  if (from.path.endsWith("/")) {
    window.sessionStorage.setItem(
      "listFrozen",
      (!to.path.endsWith("/")).toString()
    );
  } else if (to.path.endsWith("/")) {
    fileStore.updateRequest(null);
  }
  fetchData();
});
watch(reload, (newValue) => {
  newValue && fetchData();
});
watch(
  () => props.requestKey,
  () => {
    configureBrowser();
    fetchData();
  }
);
watch(
  uploadError,
  (newValue) => {
    if (newValue) $showError(newValue);
  },
  { flush: "sync" }
);

const fetchData = async () => {
  // Reset view information.
  fileStore.reload = false;
  fileStore.selected = [];
  fileStore.multiple = false;
  layoutStore.closeHovers();

  // Set loading to true and reset the error.
  if (
    window.sessionStorage.getItem("listFrozen") !== "true" &&
    window.sessionStorage.getItem("modified") !== "true"
  ) {
    layoutStore.loading = true;
  }
  error.value = null;

  let url = route.path;
  if (url === "") url = "/";
  if (url[0] !== "/") url = "/" + url;
  try {
    if (props.source === "share" || !url.endsWith("/")) {
      url += url.includes("?") ? "&presign" : "?presign";
    }
    const res = await api.fetch(url);

    const requestedPath = route.params.path;
    let requestedParts = Array.isArray(requestedPath)
      ? requestedPath
      : requestedPath
        ? [requestedPath]
        : [];
    if (props.source === "share") {
      requestedParts = requestedParts.slice(1);
    }
    const expectedPath = requestedParts.join("/");
    if (clean(res.path) !== clean(`/${expectedPath}`)) {
      throw new Error("Data Mismatch!");
    }

    fileStore.updateRequest(res);
    document.title = `${res.name} - ${t("files.files")} - ${name}`;
  } catch (err) {
    if (err instanceof Error) {
      error.value = err;
    }
  } finally {
    layoutStore.loading = false;
  }
};
const keyEvent = (event: KeyboardEvent) => {
  if (event.key === "F1") {
    event.preventDefault();
    layoutStore.showHover("help");
  }
};
</script>
