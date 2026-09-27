<template>
  <div id="login" :class="{ recaptcha: recaptcha }">
    <form
      ref="loginForm"
      method="post"
      :action="`${baseURL}/api/login`"
      autocomplete="on"
      @submit.prevent="submit"
    >
      <img :src="logoURL" class="logo-img" title="Home" />
      <h1>{{ name }}</h1>
      <div v-if="error !== ''" class="wrong">{{ error }}</div>

      <input
        v-if="loginPage"
        autofocus
        class="input input--block"
        type="text"
        id="username"
        name="username"
        autocomplete="username"
        autocapitalize="off"
        required
        v-model="username"
        :placeholder="t('login.username')"
      />
      <input
        v-if="loginPage"
        class="input input--block"
        type="password"
        id="password"
        name="password"
        autocomplete="current-password"
        required
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

    <footer id="about" aria-label="Application information">
      <div class="about-group">
        <span class="about-label">Powered by</span>
        <a
          class="about-product"
          href="https://github.com/versioneer-tech/package-r"
          target="_blank"
          rel="noopener noreferrer"
        >
          <strong>packageR</strong>
          <span class="about-version">{{ version }}</span>
        </a>
      </div>

      <span class="about-divider" aria-hidden="true"></span>

      <div class="about-group about-maintainers">
        <span class="about-label">Maintained by</span>
        <div class="about-logos">
          <a
            class="logo-link"
            href="https://versioneer.at"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="Versioneer"
          >
            <img :src="versioneerLogoURL" alt="" class="logo versioneer-logo" />
          </a>
          <a
            class="logo-link"
            href="https://eox.at"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="EOX"
          >
            <img :src="eoxLogoURL" alt="" class="logo eox-logo" />
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { StatusError } from "@/api/utils";
import * as auth from "@/utils/auth";
import {
  name,
  baseURL,
  logoURL,
  versioneerLogoURL,
  eoxLogoURL,
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
const loginForm = ref<HTMLFormElement | null>(null);

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
    if (loginPage && loginForm.value) {
      await auth.storeLoginCredential(loginForm.value);
    }
    await router.push({ path: redirect });
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
