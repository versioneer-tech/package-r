<template>
  <div class="row" v-if="!layoutStore.loading">
    <div class="column">
      <div class="card">
        <div class="card-title">
          <h2>{{ t("settings.addMember") }}</h2>
        </div>
        <div class="card-content full" v-if="members.length > 0">
          <table>
            <tr>
              <th class="padded">{{ t("settings.name") }}</th>
            </tr>
            <tr v-for="(member, index) in members" :key="index">
              <td>{{ member }}</td>
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
      <form class="card" @submit.prevent="submitAddMember" autocomplete="on">
        <div class="card-content">
          <label for="name">{{ t("member.name") }}</label>
          <input
            id="name"
            class="input input--block"
            v-model.trim="addMember.name"
            autocomplete="name"
          />
        </div>
        <div class="card-action">
          <input
            class="button button--flat"
            type="submit"
            name="submitAddMember"
            :value="t('buttons.add')"
            :disabled="layoutStore.loading"
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

interface AddMember {
  name: string;
}

const layoutStore = useLayoutStore();
const { t } = useI18n();
const $showError = inject<IToastError>("$showError")!;
const addMember = ref<AddMember>({ name: "" });
const members = ref<string[]>([]);
const ssl = window.location.protocol === "https:";
const protocol = ssl ? "wss:" : "ws:";
const authStore = useAuthStore();
const url = `${protocol}//${window.location.host}${baseURL}/api/command/?auth=${authStore.jwt}`;

const fetchMembers = () => {
  layoutStore.loading = true;
  console.log("Fetching members...");
  const ws = new WebSocket(url);

  ws.onopen = () => {
    console.log("WebSocket connected for fetching members.");
    ws.send("get-members");
  };

  ws.onmessage = (event) => {
    console.log("WebSocket message received:", event.data);
    if (event.data?.trim() && !members.value.includes(event.data.trim())) {
      members.value.push(event.data.trim());
    }
  };

  ws.onerror = (error) => {
    console.error("WebSocket error during fetch:", error);
    $showError("WebSocket connection error.");
  };

  ws.onclose = () => {
    console.log("WebSocket closed after fetching members.");
    layoutStore.loading = false;
  };
};

const submitAddMember = () => {
  layoutStore.loading = true;
  console.log("Submitting new member:", addMember.value);
  const ws = new WebSocket(url);

  ws.onopen = () => {
    console.log("WebSocket connected for submitting member.");
    ws.send(`add-member ${addMember.value.name}`);
  };

  ws.onmessage = (event) => {
    console.log(event.data);
    fetchMembers();
    addMember.value = { name: "" };
  };

  ws.onerror = (error) => {
    console.error("WebSocket error during submit:", error);
    $showError("WebSocket connection error.");
  };

  ws.onclose = () => {
    console.log("WebSocket closed after submitting member.");
    layoutStore.loading = false;
  };
};

onMounted(fetchMembers);
</script>

<style scoped>
.padded {
  padding-right: 30px;
}
</style>
