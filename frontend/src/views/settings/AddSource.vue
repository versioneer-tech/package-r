<template>
  <div class="row" v-if="!layoutStore.loading">
    <div class="column">
      <div class="card">
        <div class="card-title">
          <h2>{{ t("settings.addSource") }}</h2>
        </div>
        <div class="card-content full" v-if="sources.length > 0">
          <table>
            <tr>
              <th class="padded">{{ t("settings.membername") }}</th>
            </tr>
            <tr v-for="(source, index) in sources" :key="index">
              <td>{{ source }}</td>
            </tr>
          </table>
        </div>
        <h2 class="message" v-else>
          <i class="material-icons">sentiment_dissatisfied</i>
          <span>{{ t("files.lonely") }}</span>
        </h2>
      </div>
    </div>
    <div class="column">
      <form class="card" @submit.prevent="submitAddSource" autocomplete="on">
        <div class="card-content">
          <label for="bucketName">{{ t("source.bucketName") }}</label>
          <input
            id="bucketName"
            class="input input--block"
            v-model.trim="addSource.bucketName"
            autocomplete="bucket-name"
          />

          <label for="accessKey">{{ t("source.accessKey") }}</label>
          <input
            id="accessKey"
            class="input input--block"
            v-model.trim="addSource.accessKey"
            autocomplete="access-key"
          />

          <label for="accessSecret">{{ t("source.accessSecret") }}</label>
          <input
            id="accessSecret"
            class="input input--block"
            type="password"
            v-model.trim="addSource.accessSecret"
            autocomplete="current-password"
          />

          <label for="endpoint">{{ t("source.endpoint") }}</label>
          <input
            id="endpoint"
            class="input input--block"
            v-model.trim="addSource.endpoint"
            autocomplete="url"
          />

          <label for="region">{{ t("source.region") }}</label>
          <input
            id="region"
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
            :value="t('buttons.add')"
            disabled
            title="No s3 endpoints are whitelisted. Please contact administrator!"
          />
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useLayoutStore } from "@/stores/layout";
import { inject, ref, onMounted } from "vue";
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
const $showError = inject<IToastError>("$showError")!;
const addSource = ref<AddSource>({
  bucketName: "",
  accessKey: "",
  accessSecret: "",
  endpoint: "",
  region: "",
});
const sources = ref<string[]>([]);
const ssl = window.location.protocol === "https:";
const protocol = ssl ? "wss:" : "ws:";
const authStore = useAuthStore();
const url = `${protocol}//${window.location.host}${baseURL}/api/command/?auth=${authStore.jwt}`;

const fetchSources = () => {
  layoutStore.loading = true;
  console.log("Fetching sources...");
  const ws = new WebSocket(url);

  ws.onopen = () => {
    console.log("WebSocket connected for fetching sources.");
    ws.send("establish-sources");
  };

  ws.onmessage = (event) => {
    console.log("WebSocket message received:", event.data);
    if (event.data?.trim() && !sources.value.includes(event.data.trim())) {
      sources.value.push(event.data.trim());
    }
  };

  ws.onerror = (error) => {
    console.error("WebSocket error during fetch:", error);
    $showError("WebSocket connection error.");
  };

  ws.onclose = () => {
    console.log("WebSocket closed after fetching sources.");
    layoutStore.loading = false;
  };
};

const submitAddSource = () => {
  layoutStore.loading = true;
  console.log("Submitting new source:", addSource.value);
  const ws = new WebSocket(url);

  ws.onopen = () => {
    console.log("WebSocket connected for submitting source.");
    ws.send(
      `add-source ${addSource.value.bucketName} ${addSource.value.accessKey} ${addSource.value.accessSecret} ${addSource.value.endpoint} ${addSource.value.region}`
    );
    ws.close();
    console.log("WebSocket closed after submit.");
    fetchSources();
  };

  ws.onmessage = (event) => {
    console.log(event.data);
    fetchSources();
    addSource.value = {
      bucketName: "",
      accessKey: "",
      accessSecret: "",
      endpoint: "",
      region: "",
    };
  };

  ws.onerror = (error) => {
    console.error("WebSocket error during submit:", error);
    $showError("WebSocket connection error.");
  };

  ws.onclose = () => {
    console.log("WebSocket closed after submitting source.");
    layoutStore.loading = false;
  };
};

onMounted(fetchSources);
</script>
