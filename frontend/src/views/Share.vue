<template>
  <div>
    <header-bar showLogo>
      <title>{{ name }}</title>

      <action
        v-if="fileStore.selectedCount"
        icon="file_download"
        :label="t('buttons.download')"
        @action="download"
        :counter="fileStore.selectedCount"
      />
      <action
        icon="check_circle"
        :label="t('buttons.selectMultiple')"
        @action="toggleMultipleSelection"
      />
    </header-bar>

    <breadcrumbs :base="'/share/' + hash" />

    <div v-if="layoutStore.loading">
      <h2 class="message delayed" style="padding-top: 3em !important">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ t("files.loading") }}</span>
      </h2>
    </div>
    <div v-else-if="error">
      <div v-if="error.status === 401">
        <div class="card floating" id="password" style="z-index: 9999999">
          <div v-if="attemptedPasswordLogin" class="share__wrong__password">
            {{ t("login.wrongCredentials") }}
          </div>
          <div class="card-title">
            <h2>{{ t("login.password") }}</h2>
          </div>

          <div class="card-content">
            <input
              v-focus
              class="input input--block"
              type="password"
              :placeholder="t('login.password')"
              v-model="password"
              @keyup.enter="fetchData"
            />
          </div>
          <div class="card-action">
            <button
              class="button button--flat"
              @click="fetchData"
              :aria-label="t('buttons.submit')"
              :data-title="t('buttons.submit')"
            >
              {{ t("buttons.submit") }}
            </button>
          </div>
        </div>
        <div class="overlay" />
      </div>
      <errors v-else :errorCode="error.status" />
    </div>
    <div v-else-if="req !== null">
      <div class="share">
        <div
          class="share__box share__box__info"
          style="
            position: -webkit-sticky;
            position: sticky;
            top: -20.6em;
            z-index: 999;
          "
        >
          <div class="share__box__header" style="height: 3em">
            {{ req.isDir ? t("buttons.folder") : t("buttons.file") }}
          </div>
          <div class="share__box__element">
            <p>
              <strong>{{ $t("prompts.displayName") }}:</strong> {{ req.name }}
            </p>
            <p>
              <strong>{{ $t("prompts.size") }}:</strong> {{ humanSize }}
            </p>
            <p v-if="!req.isDir" :title="modTime">
              <strong>{{ $t("prompts.lastModified") }}:</strong> {{ humanTime }}
            </p>
            <p v-if="!req.isDir">
              <strong>MD5: </strong>
              <code>
                <a
                  @click="checksum($event, 'md5')"
                  @keypress.enter="checksum($event, 'md5')"
                  tabindex="2"
                  >{{ $t("prompts.show") }}</a
                >
              </code>
            </p>
            <p v-if="!req.isDir">
              <strong>SHA1: </strong>
              <code>
                <a
                  @click="checksum($event, 'sha1')"
                  @keypress.enter="checksum($event, 'sha1')"
                  tabindex="3"
                  >{{ $t("prompts.show") }}</a
                >
              </code>
            </p>
            <p v-if="!req.isDir">
              <strong>SHA256: </strong>
              <code>
                <a
                  @click="checksum($event, 'sha256')"
                  @keypress.enter="checksum($event, 'sha256')"
                  tabindex="4"
                  >{{ $t("prompts.show") }}</a
                >
              </code>
            </p>
            <p v-if="!req.isDir">
              <strong>SHA512: </strong>
              <code>
                <a
                  @click="checksum($event, 'sha512')"
                  @keypress.enter="checksum($event, 'sha512')"
                  tabindex="5"
                  >{{ $t("prompts.show") }}</a
                >
              </code>
            </p>
            <p v-if="!req.isDir && canPresign">
              <strong>Presigned URL: </strong>
              <code>
                <a
                  :href="presignedURL || 'javascript:void(0)'"
                  @click="presign"
                  @keypress.enter="presign"
                  tabindex="6"
                >
                  {{ presignedURL || $t("prompts.show") }}
                </a>
              </code>
            </p>
            <p v-if="!req.isDir && canPreview">
              <strong>Preview URL: </strong>
              <code>
                <a
                  :href="previewURL || 'javascript:void(0)'"
                  @click="preview"
                  @keypress.enter="preview"
                  tabindex="7"
                >
                  {{ previewURL || $t("prompts.show") }}
                </a>
              </code>
            </p>
          </div>
          <div class="share__box__element share__box__center">
            <a
              target="_blank"
              :href="link"
              class="button button--flat"
              style="height: 4em"
            >
              <div>
                <i class="material-icons">file_download</i
                >{{ t("buttons.download") }}
              </div>
            </a>
            <a
              target="_blank"
              :href="inlineLink"
              class="button button--flat"
              v-if="!req.isDir"
            >
              <div>
                <i class="material-icons">open_in_new</i
                >{{ t("buttons.openFile") }}
              </div>
            </a>
          </div>
        </div>
        <div
          id="shareList"
          v-if="req.isDir && req.items.length > 0"
          class="share__box share__box__items"
        >
          <div class="share__box__header" v-if="req.isDir">
            {{ t("files.files") }}
          </div>
          <div id="listing" class="list file-icons">
            <item
              v-for="item in req.items.slice(0, showLimit)"
              :key="base64(item.name)"
              v-bind:index="item.index"
              v-bind:name="item.name"
              v-bind:isDir="item.isDir"
              v-bind:url="item.url"
              v-bind:modified="item.modified"
              v-bind:type="item.type"
              v-bind:size="item.size"
              readOnly
            >
            </item>
            <div
              v-if="req.items.length > showLimit"
              class="item"
              @click="showLimit += 100"
            >
              <div>
                <p class="name">+ {{ req.items.length - showLimit }}</p>
              </div>
            </div>

            <div
              :class="{ active: fileStore.multiple }"
              id="multiple-selection"
            >
              <p>{{ t("files.multipleSelectionEnabled") }}</p>
              <div
                @click="() => (fileStore.multiple = false)"
                tabindex="0"
                role="button"
                :data-title="t('buttons.clear')"
                :aria-label="t('buttons.clear')"
                class="action"
              >
                <i class="material-icons">clear</i>
              </div>
            </div>
          </div>
        </div>
        <div
          v-else-if="req.isDir && req.items.length === 0"
          class="share__box share__box__items"
        >
          <h2 class="message">
            <i class="material-icons">sentiment_dissatisfied</i>
            <span>{{ t("files.lonely") }}</span>
          </h2>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { pub as pub_api } from "@/api";
import { filesize } from "@/utils";
import dayjs from "dayjs";
import { Base64 } from "js-base64";

import HeaderBar from "@/components/header/HeaderBar.vue";
import Action from "@/components/header/Action.vue";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
import Errors from "@/views/Errors.vue";
import Item from "@/components/files/ListingItem.vue";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { computed, inject, onMounted, onBeforeUnmount, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { StatusError } from "@/api/utils";
import { name, catalogPreviewURL } from "@/utils/constants";

const error = ref<StatusError | null>(null);
const showLimit = ref<number>(100);
const password = ref<string>("");
const attemptedPasswordLogin = ref<boolean>(false);
const hash = ref<string>("");
const token = ref<string>("");

const $showError = inject<IToastError>("$showError")!;

const { t } = useI18n({});

const route = useRoute();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

watch(route, () => {
  showLimit.value = 100;
  fetchData();
});

const req = computed(() => fileStore.req);

const link = computed(() =>
  req.value ? pub_api.getDownloadURL(req.value) : ""
);
const inlineLink = computed(() =>
  req.value ? pub_api.getDownloadURL(req.value, true) : ""
);
const humanSize = computed(() => {
  if (req.value) {
    return req.value.isDir
      ? req.value.items.length
      : filesize(req.value.size ?? 0);
  } else {
    return "";
  }
});
const humanTime = computed(() =>
  dayjs(req.value?.modified).isAfter("1.1.2020")
    ? dayjs(req.value?.modified).fromNow()
    : ""
);
const modTime = computed(() =>
  req.value
    ? new Date(Date.parse(req.value.modified)).toLocaleString()
    : new Date().toLocaleString()
);

const base64 = (name: any) => Base64.encodeURI(name);
const fetchData = async () => {
  fileStore.reload = false;
  fileStore.selected = [];
  fileStore.multiple = false;

  presignedURL.value = null;
  previewURL.value = null;

  layoutStore.closeHovers();

  // Set loading to true and reset the error.
  layoutStore.loading = true;
  error.value = null;
  if (password.value !== "") {
    attemptedPasswordLogin.value = true;
  }

  let url = route.path;
  if (url === "") url = "/";
  if (url[0] !== "/") url = "/" + url;

  try {
    const file = await pub_api.fetch(url, password.value);
    file.hash = hash.value;

    token.value = file.token || "";

    fileStore.updateRequest(file);
    document.title = `${file.name} - ${document.title}`;
  } catch (err) {
    if (err instanceof Error) {
      error.value = err;
    }
  } finally {
    layoutStore.loading = false;
  }
};

const keyEvent = (event: KeyboardEvent) => {
  if (event.key === "Escape") {
    // If we're on a listing, unselect all
    // files and folders.
    if (fileStore.selectedCount > 0) {
      fileStore.selected = [];
    }
  }
};

const toggleMultipleSelection = () => {
  fileStore.toggleMultiple();
};

const isSingleFile = () =>
  fileStore.selectedCount === 1 &&
  !req.value?.items[fileStore.selected[0]].isDir;

const download = () => {
  if (!req.value) return false;

  if (isSingleFile()) {
    pub_api.download(
      null,
      hash.value,
      token.value,
      req.value.items[fileStore.selected[0]].path
    );
    return true;
  }

  layoutStore.showHover({
    prompt: "download",
    confirm: (format: DownloadFormat) => {
      if (req.value === null) return false;
      layoutStore.closeHovers();

      const files: string[] = [];

      for (const i of fileStore.selected) {
        files.push(req.value.items[i].path);
      }

      pub_api.download(format, hash.value, token.value, ...files);
      return true;
    },
  });

  return true;
};

onMounted(async () => {
  hash.value = route.params.path[0];
  window.addEventListener("keydown", keyEvent);
  await fetchData();

  canPresign.value = true; // TBD
  canPreview.value = catalogPreviewURL !== "";
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", keyEvent);
});

const presignedURL = ref<string | null>(null);
const previewURL = ref<string | null>(null);
const canPresign = ref<boolean>(false);
const canPreview = ref<boolean>(false);

const checksum = async (event: Event, algo: string) => {
  event.preventDefault();

  let link: string;

  if (fileStore.selectedCount) {
    link = req.value?.items[fileStore.selected[0]].url || route.path;
  } else {
    link = route.path;
  }

  try {
    const hash = await pub_api.checksum(link, algo);
    (event.target as HTMLElement).textContent = hash;
  } catch (e) {
    if (e instanceof Error) {
      $showError(e);
    } else {
      $showError(String(e));
    }
  }
};

const presign = async (event: Event) => {
  if (typeof presignedURL.value === "string") {
    return;
  }

  event.preventDefault();

  const link = fileStore.selectedCount
    ? req.value?.items[fileStore.selected[0]].url
    : route.path;

  try {
    const value = await pub_api.presign(link || "");
    presignedURL.value = value;
  } catch (e) {
    if (e instanceof Error) {
      $showError(e);
    } else {
      $showError(String(e));
    }
  }
};

const preview = async (event: Event) => {
  if (typeof previewURL.value === "string") {
    return;
  }

  event.preventDefault();

  const link = fileStore.selectedCount
    ? req.value?.items[fileStore.selected[0]].url
    : route.path;

  try {
    const value = await pub_api.preview(link || "");
    previewURL.value = value;
  } catch (e) {
    if (e instanceof Error) {
      $showError(e);
    } else {
      $showError(String(e));
    }
  }
};
</script>

<style scoped>
#listing.list {
  height: auto;
}

#shareList {
  overflow-y: scroll;
}

@media (min-width: 930px) {
  #shareList {
    height: calc(100vh - 9.8em);
    overflow-y: auto;
  }
}
</style>
