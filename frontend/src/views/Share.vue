<template>
  <Files source="share" :password="password" :request-key="requestKey">
    <template #error="{ error }">
      <div v-if="error.status === 401">
        <div class="card floating share-password" id="password">
          <div v-if="attemptedPasswordLogin" class="share__wrong__password">
            {{ t("login.wrongCredentials") }}
          </div>
          <div class="card-title">
            <h2>{{ t("login.password") }}</h2>
          </div>

          <div class="card-content">
            <input
              v-focus
              class="input input--block"
              type="password"
              :placeholder="t('login.password')"
              v-model="passwordInput"
              @keyup.enter="submitPassword"
            />
          </div>
          <div class="card-action">
            <button
              class="button button--flat"
              @click="submitPassword"
              :aria-label="t('buttons.submit')"
              :data-title="t('buttons.submit')"
            >
              {{ t("buttons.submit") }}
            </button>
          </div>
        </div>
        <div class="overlay" />
      </div>
      <Errors v-else :errorCode="error.status" />
    </template>
  </Files>
</template>

<script setup lang="ts">
import Files from "@/views/Files.vue";
import Errors from "@/views/Errors.vue";
import { ref } from "vue";
import { useI18n } from "vue-i18n";

const { t } = useI18n();

const attemptedPasswordLogin = ref(false);
const password = ref("");
const passwordInput = ref("");
const requestKey = ref(0);

const submitPassword = () => {
  attemptedPasswordLogin.value = true;
  password.value = passwordInput.value;
  requestKey.value++;
};
</script>
