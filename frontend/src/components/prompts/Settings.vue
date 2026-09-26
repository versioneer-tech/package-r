<template>
  <div class="card floating" id="settings">
    <div class="card-title">
      <h2>{{ t("sidebar.settings") }}</h2>
    </div>

    <div class="card-content">
      <h3>{{ t("prompts.availableShares") }}</h3>
      <p v-if="loading">{{ t("files.loading") }}</p>
      <ul v-else-if="shares.length > 0" id="settings-shares">
        <li v-for="share in shares" :key="share.hash">
          <a :href="buildLink(share)" target="_blank" rel="noopener noreferrer">
            <strong>{{ share.hash }}</strong>
            <span v-if="share.description"> — {{ share.description }}</span>
          </a>
          <button
            class="action"
            :aria-label="t('buttons.copyToClipboard')"
            :title="t('buttons.copyToClipboard')"
            @click="copyToClipboard(buildLink(share))"
          >
            <i class="material-icons">content_copy</i>
          </button>
        </li>
      </ul>
      <p v-else>{{ t("prompts.noShares") }}</p>
    </div>

    <div class="card-action">
      <button
        id="focus-prompt"
        class="button button--flat button--grey"
        @click="layoutStore.closeHovers"
      >
        {{ t("buttons.close") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { settings as api } from "@/api";
import { useLayoutStore } from "@/stores/layout";
import { copy } from "@/utils/clipboard";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

const layoutStore = useLayoutStore();
const shares = ref<ConfiguredShare[]>([]);
const loading = ref(true);
const { t } = useI18n();
const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

onMounted(async () => {
  try {
    shares.value = await api.getShares();
  } catch (error) {
    $showError(error as Error);
  } finally {
    loading.value = false;
  }
});

const buildLink = (share: ConfiguredShare) => api.getShareURL(share);

const copyToClipboard = async (text: string) => {
  try {
    await copy({ text });
    $showSuccess(t("success.linkCopied"));
  } catch {
    try {
      await copy({ text }, { permission: true });
      $showSuccess(t("success.linkCopied"));
    } catch (error) {
      $showError(error as Error);
    }
  }
};
</script>
