<template>
  <div>
    <header-bar
      v-if="error || fileStore.req?.type === null"
      showMenu
      showLogo
    />

    <breadcrumbs base="/files" />
    <errors v-if="error" :errorCode="error.status" />
    <component v-else-if="currentView && currentView !== IframeRenderer" :is="currentView"></component>
    <IframeRenderer v-else-if="currentView === IframeRenderer" :src="fileStore.req?.presignedURL" />
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
  ref,
  watch,
  defineComponent,
  h,
} from "vue";
import { files as api } from "@/api";
import { storeToRefs } from "pinia";
import { useFileStore } from "@/stores/file";
import { useAuthStore } from "@/stores/auth";
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

const authStore = useAuthStore();
const layoutStore = useLayoutStore();
const fileStore = useFileStore();
const uploadStore = useUploadStore();

const { reload } = storeToRefs(fileStore);
const { error: uploadError } = storeToRefs(uploadStore);

const route = useRoute();
const { t } = useI18n({});

const clean = (path: string) => {
  return path.endsWith("/") ? path.slice(0, -1) : path;
};

const error = ref<StatusError | null>(null);

const IframeRenderer = defineComponent({
  name: "IframeRenderer",
  props: {
    src: {
      type: String,
      required: false,
    },
  },
  setup(props) {
    const loadError = ref(false);
    const checked = ref(false);

    const probePresignedURL = async () => {
      if (!props.src) {
        loadError.value = true;
        checked.value = true;
        return;
      }

      try {
        const res = await fetch(props.src, {
          method: "GET",
          headers: {
            Range: "bytes=0-0",
          },
        });

        if (!res.ok) {
          console.warn("Presigned URL probe failed with status:", res.status);
          loadError.value = true;
        }
      } catch (e) {
        console.warn("Presigned URL probe failed:", e);
        loadError.value = true;
      } finally {
        checked.value = true;
      }
    };

    onMounted(probePresignedURL);

    return () =>
      !checked.value
        ? h("div", { style: "text-align: center; padding: 2em;" }, "Loading...")
        : !props.src || loadError.value
        ? h(Errors, { errorCode: 415 })
        : h("div", { style: "padding: 1em;" }, [
            h("iframe", {
              src: props.src,
              style: "width: 100%; height: 80vh; border: none;",
              loading: "lazy",
            }),
          ]);
  },
});

const currentView = computed(() => {
  const req = fileStore.req;
  const user = authStore.user;

  if (!req || req.type === undefined) {
    return null;
  }

  if (req.isDir) {
    return FileListing;
  }

  if (req.type === "text" || req.type === "textImmutable") {
    return Editor;
  }

  if (req.type === "pdf" || req.type === "image" || req.type === "audio" || req.type === "video") {
    return Preview;
  }

  if (req.type === "tiff") {
    return IframeRenderer; // TBD
  }

  if (req.type === "parquet") {
    return IframeRenderer; // TBD
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

// Define hooks
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
});

watch(route, (to, from) => {
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
watch(uploadError, (newValue) => {
  newValue && layoutStore.showError();
});

// Define functions

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
    if (!url.endsWith("/")) {
      url += url.includes("?") ? "&presign" : "?presign";
    }    
    const res = await api.fetch(url);
    console.log(res)

    if (clean(res.path) !== clean(`/${[...route.params.path].join("/")}`)) {
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
