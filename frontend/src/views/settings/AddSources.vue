<template>
  <div class="row">
    <div class="column">
      <form class="card" @submit="addSource">
        <div class="card-title">
          <h2>{{ t("settings.addSources") }}</h2>
        </div>

        <div class="card-content">
          <p>{{ $t("source.name") }}</p>
          <input class="input input--block" v-model.trim="source.name" />
          <p>{{ $t("source.bucketName") }}</p>
          <input class="input input--block" v-model.trim="source.bucketName" />
          <p>{{ $t("source.endpoint") }}</p>
          <input class="input input--block" v-model.trim="source.endpoint" />
          <p>{{ $t("source.region") }}</p>
          <input class="input input--block" v-model.trim="source.region" />
          <p>{{ $t("source.credentials") }}</p>
          <input class="input input--block" v-model.trim="source.credentials" />
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
import { source as api } from "@/api";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { NewSource } from "@/types/sources.js";

const layoutStore = useLayoutStore();
const { t } = useI18n();

// const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const $showError = inject<IToastError>("$showError")!;

const source = ref<NewSource>({} as NewSource);

onMounted(() => {
  layoutStore.loading = true;
  // add initialisation
  layoutStore.loading = false;
  return true;
});

const addSource = async (event: Event) => {
  event.preventDefault();

  try {
    await api.update(source.value);
    //    $showSuccess(t("settings.settingsUpdated"));
  } catch (err) {
    if (err instanceof Error) {
      $showError(err);
    }
  }
};
</script>
