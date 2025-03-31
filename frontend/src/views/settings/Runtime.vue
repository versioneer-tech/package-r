<template>
  <div class="row">
    <div class="column">
      <form class="card" @submit.prevent="updateRuntimeStatus">
        <div class="card-title">
          <h2>{{ t("settings.runtime") }}</h2>
        </div>

        <div class="card-content">
          <div v-for="runtime in runtimes" :key="runtime.type">
            <h3>{{ runtime.type }}</h3>
            <select
              v-model="runtime.status"
              :disabled="runtime.status === 'disabled'"
              class="input input--block"
            >
              <option value="active">Active</option>
              <option value="suspended">Suspended</option>
              <option value="disabled">Disabled</option>
            </select>
          </div>
        </div>

        <div class="card-action">
          <input
            class="button button--flat"
            type="submit"
            :value="t('buttons.update')"
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

interface Runtime {
  type: string;
  status: "active" | "suspended" | "disabled";
}

const layoutStore = useLayoutStore();
const { t } = useI18n();
const $showError = inject<IToastError>("$showError")!;
const runtimes = ref<Runtime[]>([]);
const ssl = window.location.protocol === "https:";
const protocol = ssl ? "wss:" : "ws:";
const authStore = useAuthStore();
const url = `${protocol}//${window.location.host}${baseURL}/api/command/?auth=${authStore.jwt}`;

const fetchRuntimeStatus = () => {
  layoutStore.loading = true;
  console.log("Fetching runtime status...");
  runtimes.value = [];
  const ws = new WebSocket(url);

  ws.onopen = () => {
    console.log("WebSocket connected for fetching runtimes.");
    ws.send("get-runtime-status");
  };

  ws.onmessage = (event) => {
    console.log("WebSocket message received:", event.data);
    if (typeof event.data === "string") {
      const [type, status] = event.data.split("=").map((s: string) => s.trim()); // Add type annotation here
      if (!runtimes.value.some((runtime) => runtime.type === type)) {
        runtimes.value.push({ type, status } as Runtime);
      }
    }
  };

  ws.onerror = (error) => {
    console.error("WebSocket error during fetch:", error);
    $showError("WebSocket connection error.");
  };

  ws.onclose = () => {
    console.log("WebSocket closed after fetching runtimes.");
    layoutStore.loading = false;
  };
};

const updateRuntimeStatus = async () => {
  const ws = new WebSocket(url);
  ws.onopen = () => {
    console.log("WebSocket connected for updating runtime status.");
    runtimes.value.forEach((runtime) => {
      if (runtime.status !== "disabled") {
        ws.send(`set-runtime-status ${runtime.type} ${runtime.status}`);
      }
    });
  };

  ws.onmessage = (event) => {
    console.log("Response:", event.data);
  };

  ws.onerror = (error) => {
    console.error("WebSocket error during update:", error);
    $showError("WebSocket connection error.");
  };

  ws.onclose = () => {
    console.log("WebSocket closed after updating runtime status.");
    fetchRuntimeStatus();
  };
};

onMounted(fetchRuntimeStatus);
</script>
