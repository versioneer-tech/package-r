<template>
  <div class="settings-page">
    <header-bar show-menu show-logo />

    <section class="settings-content">
      <form id="profile-settings" class="card" @submit.prevent="save">
        <div class="card-title">
          <h2>{{ t("settings.profileSettings") }}</h2>
        </div>

        <div class="card-content profile-fields">
          <label>
            <input v-model="hideDotfiles" type="checkbox" name="hideDotfiles" />
            <span>{{ t("settings.hideDotfiles") }}</span>
          </label>
          <label>
            <input v-model="singleClick" type="checkbox" name="singleClick" />
            <span>{{ t("settings.singleClick") }}</span>
          </label>
          <label>
            <input v-model="dateFormat" type="checkbox" name="dateFormat" />
            <span>{{ t("settings.setDateFormat") }}</span>
          </label>

          <label class="language-field">
            <span>{{ t("settings.language") }}</span>
            <languages v-model:locale="locale" class="input" />
          </label>
        </div>

        <div class="card-action">
          <button class="button button--flat" type="submit">
            {{ t("buttons.save") }}
          </button>
        </div>
      </form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { settings as api } from "@/api";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Languages from "@/components/settings/Languages.vue";
import { useAuthStore } from "@/stores/auth";
import { inject, ref } from "vue";
import { useI18n } from "vue-i18n";

const authStore = useAuthStore();
const { t } = useI18n();
const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const locale = ref(authStore.user?.locale || "en");
const hideDotfiles = ref(authStore.user?.hideDotfiles || false);
const singleClick = ref(authStore.user?.singleClick || false);
const dateFormat = ref(authStore.user?.dateFormat || false);

const save = async () => {
  const profile = {
    locale: locale.value,
    hideDotfiles: hideDotfiles.value,
    singleClick: singleClick.value,
    dateFormat: dateFormat.value,
  };

  try {
    await api.updateProfile(profile);
    authStore.updateUser(profile);
    $showSuccess(t("settings.settingsUpdated"));
  } catch (error) {
    $showError(error as Error);
  }
};
</script>

<style scoped>
.settings-content {
  width: 100%;
}

.profile-fields {
  display: grid;
  gap: 1rem;
}

.profile-fields label {
  align-items: center;
  display: flex;
  gap: 0.75rem;
}

.language-field {
  align-items: flex-start !important;
  flex-direction: column;
}

.language-field select {
  width: 100%;
}
</style>
