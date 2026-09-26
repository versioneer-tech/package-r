<template>
  <div id="login" :class="{ recaptcha: recaptcha }">
    <form @submit="submit">
      <img :src="logoURL" class="logo-img" title="Home" />
      <h1>{{ name }}</h1>
      <div v-if="error !== ''" class="wrong">{{ error }}</div>

      <input
        v-if="loginPage"
        autofocus
        class="input input--block"
        type="text"
        autocapitalize="off"
        v-model="username"
        :placeholder="t('login.username')"
      />
      <input
        v-if="loginPage"
        class="input input--block"
        type="password"
        v-model="password"
        :placeholder="t('login.password')"
      />
      <div v-if="recaptcha" id="recaptcha"></div>
      <input
        class="button button--block"
        type="submit"
        :value="t('login.submit')"
      />
    </form>

    <div id="about">
      <p>
        Powered by
        <a href="https://github.com/versioneer-tech/package-r" target="_blank">
          <strong>packageR</strong>
        </a>
        {{ version }}, maintained by
        <a href="https://versioneer.at" target="_blank">
          <img
            src="https://raw.githubusercontent.com/versioneer-inc/versioneer-inc.github.io/master/logo_versioneer_white.png"
            alt="Versioneer"
            class="logo versioneer-logo"
          />
        </a>
        and
        <a href="https://eox.at" target="_blank">
          <img
            src="https://eox.at/EOX_Logo.svg"
            alt="EOX"
            class="logo eox-logo"
          />
        </a>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { StatusError } from "@/api/utils";
import * as auth from "@/utils/auth";
import {
  name,
  logoURL,
  recaptcha,
  recaptchaKey,
  loginPage,
  version,
} from "@/utils/constants";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

const error = ref<string>("");
const username = ref<string>("");
const password = ref<string>("");

const route = useRoute();
const router = useRouter();
const { t } = useI18n({});
const $showError = inject<IToastError>("$showError")!;

const submit = async (event: Event) => {
  event.preventDefault();
  event.stopPropagation();

  const redirect = (route.query.redirect || "/files/") as string;

  let captcha = "";
  if (recaptcha) {
    captcha = window.grecaptcha.getResponse();

    if (captcha === "") {
      error.value = t("login.wrongCredentials");
      return;
    }
  }

  try {
    await auth.login(username.value, password.value, captcha);
    router.push({ path: redirect });
  } catch (e: any) {
    if (e instanceof StatusError) {
      if (e.status === 403) {
        error.value = t("login.wrongCredentials");
      } else {
        error.value = t("login.notPossible");
      }
    } else {
      $showError(e);
    }
  }
};

onMounted(() => {
  if (!recaptcha) return;

  window.grecaptcha.ready(function () {
    window.grecaptcha.render("recaptcha", {
      sitekey: recaptchaKey,
    });
  });
});
</script>
