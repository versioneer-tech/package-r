<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="row" v-else-if="!layoutStore.loading">
    <div class="column">
      <div class="card">
        <div class="card-title">
          <h2>{{ t("settings.shareManagement") }}</h2>
        </div>

        <div class="card-content full">
          <table>
            <thead>
              <tr>
                <th>{{ t("settings.hash") }}</th>
                <th>{{ t("settings.path") }}</th>
                <th>{{ t("settings.shareDescription") }}</th>
                <th>{{ t("settings.shareDuration") }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="link in links" :key="link.hash">
                <td>
                  <a
                    :href="shareApi.getShareURL(link)"
                    target="_blank"
                    rel="noopener noreferrer"
                    >{{ link.hash }}</a
                  >
                </td>
                <td>{{ link.path }}</td>
                <td>{{ link.description }}</td>
                <td>{{ expiration(link.expire) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { share as shareApi } from "@/api";
import { StatusError } from "@/api/utils";
import { useLayoutStore } from "@/stores/layout";
import Errors from "@/views/Errors.vue";

const error = ref<StatusError | null>(null);
const links = ref<ConfiguredShare[]>([]);
const layoutStore = useLayoutStore();
const { t } = useI18n();

const expiration = (expire: number) => {
  if (expire === 0) return t("permanent");
  return new Date(expire * 1000).toLocaleString();
};

onMounted(async () => {
  layoutStore.loading = true;
  try {
    links.value = await shareApi.list();
  } catch (err) {
    if (err instanceof StatusError) error.value = err;
  } finally {
    layoutStore.loading = false;
  }
});
</script>
