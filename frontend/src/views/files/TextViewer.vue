<template>
  <div id="text-viewer-container">
    <header-bar>
      <action
        v-if="canClose"
        icon="close"
        :label="t('buttons.close')"
        @action="close"
      />
      <title>{{ fileStore.req?.name ?? "" }}</title>
    </header-bar>

    <Breadcrumbs :base="browserStore.basePath" noLink />
    <section
      v-if="isMarkdown"
      class="md_preview markdown-viewer"
      data-testid="markdown-viewer"
      tabindex="0"
      v-html="renderedMarkdown"
    ></section>
    <pre v-else data-testid="text-viewer" tabindex="0">{{ textContent }}</pre>
  </div>
</template>

<script setup lang="ts">
import Action from "@/components/header/Action.vue";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
import { useBrowserStore } from "@/stores/browser";
import { useFileStore } from "@/stores/file";
import url from "@/utils/url";
import DOMPurify from "dompurify";
import { marked } from "marked";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

const browserStore = useBrowserStore();
const fileStore = useFileStore();
const route = useRoute();
const router = useRouter();
const { t } = useI18n();

const content = computed(() => fileStore.req?.content ?? "");
const extension = computed(() => fileStore.req?.extension.toLowerCase() ?? "");
const isMarkdown = computed(() =>
  [".md", ".markdown"].includes(extension.value)
);
const isJson = computed(() => extension.value === ".json");

const renderedMarkdown = computed(() =>
  DOMPurify.sanitize(marked.parse(content.value, { async: false }), {
    USE_PROFILES: { html: true },
  })
);

const textContent = computed(() => {
  if (!isJson.value) return content.value;

  try {
    return JSON.stringify(JSON.parse(content.value), null, 2);
  } catch {
    return content.value;
  }
});

const canClose = computed(
  () => route.path.replace(/\/$/, "") !== browserStore.basePath
);

const close = () => {
  if (!canClose.value) return;

  fileStore.updateRequest(null);
  router.push({ path: url.removeLastDir(route.path) + "/" });
};
</script>

<style scoped>
#text-viewer-container {
  display: flex;
  flex-direction: column;
  background: var(--background);
  position: fixed;
  padding-top: 4em;
  inset: 0;
  z-index: 9998;
  overflow: hidden;
}

#text-viewer-container :deep(.bar) {
  background: var(--surfacePrimary);
}

#text-viewer-container :deep(.breadcrumbs) {
  height: 2.3em;
  padding: 0 1em;
}

pre {
  box-sizing: border-box;
  flex: 1;
  width: 100%;
  min-height: 0;
  margin: 0;
  padding: 1rem;
  overflow: auto;
  color: var(--textPrimary);
  background: var(--background);
  font-family: monospace;
  font-size: 0.875rem;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.markdown-viewer {
  box-sizing: border-box;
  flex: 1;
  width: 100%;
  min-height: 0;
  max-height: none;
  margin: 0;
  overflow: auto;
  color: var(--textPrimary);
  background: var(--background);
}
</style>
