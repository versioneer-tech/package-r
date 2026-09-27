<template>
  <header>
    <img
      v-if="showLogo"
      :src="logoURL"
      @click="$router.push({ path: '/' })"
      class="logo-img"
      title="Home"
    />
    <Action
      v-if="showMenu && authStore.user"
      icon="settings"
      :label="t('settings.profileSettings')"
      @action="openProfileSettings"
    />
    <Action
      v-if="showMenu && authStore.user"
      icon="link"
      :label="t('settings.shareManagement')"
      @action="openShareManagement"
    />
    <Action
      v-if="showMenu && authStore.user"
      icon="exit_to_app"
      :label="t('sidebar.logout')"
      @action="auth.logout"
    />

    <slot />

    <div
      id="dropdown"
      :class="{ active: layoutStore.currentPromptName === 'more' }"
    >
      <slot name="actions" />
    </div>

    <Action
      v-if="ifActionsSlot"
      id="more"
      icon="more_vert"
      :label="t('buttons.more')"
      @action="layoutStore.showHover('more')"
    />

    <div
      class="overlay"
      v-show="layoutStore.currentPromptName == 'more'"
      @click="layoutStore.closeHovers"
    />
  </header>
</template>

<script setup lang="ts">
import { useLayoutStore } from "@/stores/layout";
import { useAuthStore } from "@/stores/auth";
import { logoURL } from "@/utils/constants";

import Action from "@/components/header/Action.vue";
import { computed, useSlots } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import * as auth from "@/utils/auth";

defineProps<{
  showLogo?: boolean;
  showMenu?: boolean;
}>();

const authStore = useAuthStore();
const layoutStore = useLayoutStore();
const router = useRouter();
const slots = useSlots();

const { t } = useI18n();

const ifActionsSlot = computed(() => (slots.actions ? true : false));

const openProfileSettings = () => {
  layoutStore.closeHovers();
  router.push("/settings");
};

const openShareManagement = () => {
  layoutStore.closeHovers();
  router.push("/shares");
};
</script>

<style scoped>
.logo-img {
  cursor: pointer;
  transition: transform 0.2s ease;
}
.logo-img:hover {
  transform: scale(1.05);
}
</style>
