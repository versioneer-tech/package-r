<template>
  <div class="row">
    <div class="column">
      <form class="card" @submit="submitAddSource">
        <div class="card-title">
          <h2>{{ t("settings.addSources") }}</h2>
        </div>

        <div class="card-content">
          <p>{{ $t("source.bucketName") }}</p>
          <input
            class="input input--block"
            v-model.trim="addSource.bucketName"
          />
          <p>{{ $t("source.accessKey") }}</p>
          <input
            class="input input--block"
            v-model.trim="addSource.accessKey"
          />
          <p>{{ $t("source.accessSecret") }}</p>
          <input
            class="input input--block"
            v-model.trim="addSource.accessSecret"
          />
          <p>{{ $t("source.endpoint") }}</p>
          <input class="input input--block" v-model.trim="addSource.endpoint" />
          <p>{{ $t("source.region") }}</p>
          <input class="input input--block" v-model.trim="addSource.region" />
        </div>

        <div class="card-action">
          <input
            class="button button--flat"
            type="submit"
            name="submitAddSource"
            :value="t('buttons.save')"
          />
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useLayoutStore } from "@/stores/layout";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { baseURL } from "@/utils/constants";
import { useAuthStore } from "@/stores/auth";

interface AddSource {
  bucketName: string;
  accessKey: string;
  accessSecret: string;
  endpoint: string;
  region: string;
}

const layoutStore = useLayoutStore();
const { t } = useI18n();

const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const $showError = inject<IToastError>("$showError")!;

const addSource = ref<AddSource>({} as AddSource);

onMounted(() => {
  layoutStore.loading = true;
  // add initialisation
  layoutStore.loading = false;
  return true;
});

const ssl = window.location.protocol === "https:";
const protocol = ssl ? "wss:" : "ws:";

const submitAddSource = async (event: Event) => {
  event.preventDefault();
  try {
    const authStore = useAuthStore();
    const url = `${protocol}//${window.location.host}${baseURL}/api/command/?auth=${authStore.jwt}`;

    const websocket = await new Promise<WebSocket>((resolve, reject) => {
      const conn = new WebSocket(url);

      conn.onopen = () => {
        resolve(conn);
      };

      conn.onerror = (error) => {
        reject(error);
      };
    });

    websocket.send(
      `add-source ${addSource.value.bucketName} ${addSource.value.accessKey} ${addSource.value.accessSecret} ${addSource.value.endpoint} ${addSource.value.region}`
    );
    $showSuccess(`Source added`);
    websocket.close();
  } catch (error) {
    if (error instanceof Error) {
      $showError(error);
    }
  }
};
</script>
