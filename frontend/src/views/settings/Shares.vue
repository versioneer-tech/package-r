<template>
  <div class="settings-page">
    <header-bar show-menu show-logo />

    <section class="settings-content">
      <div id="settings-shares" class="card">
        <div class="card-title">
          <h2>{{ t("settings.shareManagement") }}</h2>
        </div>

        <div class="card-content full">
          <p v-if="loading">{{ t("files.loading") }}</p>
          <p v-else-if="shares.length === 0">{{ t("prompts.noShares") }}</p>
          <table v-else>
            <thead>
              <tr>
                <th>{{ t("settings.source") }}</th>
                <th>{{ t("settings.path") }}</th>
                <th>{{ t("settings.shareDuration") }}</th>
                <th>{{ t("settings.shareDescription") }}</th>
                <th>{{ t("settings.catalog") }}</th>
                <th>{{ t("settings.assetMappings") }}</th>
                <th>{{ t("settings.passwordProtected") }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="share in shares" :key="share.hash">
                <td>{{ share.source }}</td>
                <td>
                  <a
                    class="share-path"
                    :href="buildLink(share)"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    <code>{{ share.path }}</code>
                  </a>
                </td>
                <td>{{ expiration(share.expire) }}</td>
                <td>{{ share.description || "—" }}</td>
                <td>
                  <code>{{ share.catalog || "—" }}</code>
                </td>
                <td>
                  <code>{{ assetMappings(share.assetMappings) }}</code>
                </td>
                <td>
                  {{
                    share.passwordProtected
                      ? t("settings.yes")
                      : t("settings.no")
                  }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { settings as api } from "@/api";
import HeaderBar from "@/components/header/HeaderBar.vue";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

const shares = ref<ConfiguredShare[]>([]);
const loading = ref(true);
const { t } = useI18n();
const $showError = inject<IToastError>("$showError")!;

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

const expiration = (expire: number) => {
  if (expire === 0) return t("permanent");
  return new Date(expire * 1000).toLocaleString();
};

const assetMappings = (mappings?: CatalogAssetMapping[]) => {
  if (!mappings || mappings.length === 0) return "—";
  return JSON.stringify(mappings);
};
</script>

<style scoped>
.settings-content {
  width: 100%;
}

table {
  border-collapse: collapse;
  width: 100%;
}

th,
td {
  border-bottom: 1px solid var(--borderPrimary);
  padding: 0.75rem;
  text-align: left;
  vertical-align: top;
}

td code {
  overflow-wrap: anywhere;
}

.share-path {
  color: var(--blue);
}

@media (max-width: 600px) {
  table {
    min-width: 56rem;
  }
}
</style>
