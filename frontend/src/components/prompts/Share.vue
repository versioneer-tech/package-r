<template>
  <div class="card floating" id="share">
    <div class="card-title">
      <h2>{{ $t("buttons.share") }}</h2>
    </div>

    <div class="card-content">
      <p v-if="loading">{{ $t("files.loading") }}</p>
      <ul v-else-if="links.length > 0">
        <li v-for="link in links" :key="link.hash">
          <a :href="buildLink(link)" target="_blank" rel="noopener noreferrer">
            <strong>{{ link.hash }}</strong>
            <span v-if="link.description"> — {{ link.description }}</span>
          </a>
          <button
            class="action copy-clipboard"
            :aria-label="$t('buttons.copyToClipboard')"
            :title="$t('buttons.copyToClipboard')"
            @click="copyToClipboard(buildLink(link))"
          >
            <i class="material-icons">content_paste</i>
          </button>
        </li>
      </ul>
      <p v-else>{{ $t("files.lonely") }}</p>
    </div>

    <div class="card-action">
      <button
        id="focus-prompt"
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="$t('buttons.close')"
        :title="$t('buttons.close')"
      >
        {{ $t("buttons.close") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { storeToRefs } from "pinia";
import { share as api } from "@/api";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { copy } from "@/utils/clipboard";
import { useI18n } from "vue-i18n";

const fileStore = useFileStore();
const layoutStore = useLayoutStore();
const route = useRoute();
const { t } = useI18n();
const { selected, selectedCount } = storeToRefs(fileStore);
const links = ref<Share[]>([]);
const loading = ref(true);

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const selectedURL = computed(() => {
  if (selectedCount.value !== 1 || !fileStore.req?.isDir) {
    return route.path;
  }
  return fileStore.req.items[selected.value[0]].url;
});

onMounted(async () => {
  try {
    links.value = await api.get(selectedURL.value);
  } catch (error) {
    if (error instanceof Error) {
      $showError(error);
    }
  } finally {
    loading.value = false;
  }
});

const buildLink = (link: Share) => api.getShareURL(link);

const copyToClipboard = async (text: string) => {
  try {
    await copy({ text });
    $showSuccess(t("success.linkCopied"));
  } catch {
    try {
      await copy({ text }, { permission: true });
      $showSuccess(t("success.linkCopied"));
    } catch (permissionError) {
      $showError(permissionError as Error);
    }
  }
};

const closeHovers = () => layoutStore.closeHovers();
</script>
