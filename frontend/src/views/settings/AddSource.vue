<template>
  <div class="row">
    <div class="column">
      <form class="card" @submit="submitAddSource" autocomplete="on">
        <div class="card-title">
          <h2>{{ t("settings.addSource") }}</h2>
        </div>

        <div class="card-content">
          <p>{{ $t("source.bucketName") }}</p>
          <input
            class="input input--block"
            v-model.trim="addSource.bucketName"
            autocomplete="bucket-name"
          />
          <p>{{ $t("source.accessKey") }}</p>
          <input
            class="input input--block"
            v-model.trim="addSource.accessKey"
            autocomplete="access-key"
          />
          <p>{{ $t("source.accessSecret") }}</p>
          <input
            class="input input--block"
            type="password"
            v-model.trim="addSource.accessSecret"
            autocomplete="current-password"
          />
          <p>{{ $t("source.endpoint") }}</p>
          <input
            class="input input--block"
            v-model.trim="addSource.endpoint"
            autocomplete="url"
          />
          <p>{{ $t("source.region") }}</p>
          <input
            class="input input--block"
            v-model.trim="addSource.region"
            autocomplete="region"
          />
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
  layoutStore.loading = false;
  return true;
});

const submitAddSource = async (event: Event) => {
  event.preventDefault();
  const ssl = window.location.protocol === "https:";
  const protocol = ssl ? "wss:" : "ws:";
  const authStore = useAuthStore();
  const url = `${protocol}//${window.location.host}${baseURL}/api/command/?auth=${authStore.jwt}`;

  try {
    const websocket = new WebSocket(url);

    websocket.onopen = () => {
      console.log("WebSocket connection opened");
      websocket.send(`add-source ${addSource.value.bucketName} ${addSource.value.accessKey} ${addSource.value.accessSecret} ${addSource.value.endpoint} ${addSource.value.region}`);
    };

    websocket.onmessage = (event) => {
      console.log("Message received from server:", event.data);
      if (event.data) {
        $showSuccess(`Source added`);
      }
    };

    websocket.onerror = (error) => {
      console.error("WebSocket error:", error);
      $showError("WebSocket error occurred");
    };

    websocket.onclose = () => {
      console.log("WebSocket connection closed");
    };
  } catch (error) {
    console.error("Error creating WebSocket:", error);
    $showError("An error occurred while creating WebSocket");
  }
};
</script>
