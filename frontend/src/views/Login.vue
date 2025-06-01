<template>
  <div id="login" :class="{ recaptcha: recaptcha }">
    <form @submit="submit">
      <img :src="logoURL" class="logo-img" title="Home" />
      <h1>{{ name }}</h1>
      <div v-if="error !== ''" class="wrong">{{ error }}</div>

      <input
        v-if="loginPage || createMode"
        autofocus
        class="input input--block"
        type="text"
        autocapitalize="off"
        v-model="username"
        :placeholder="t('login.username')"
      />
      <input
        v-if="loginPage || createMode"
        class="input input--block"
        type="password"
        v-model="password"
        :placeholder="t('login.password')"
      />
      <input
        class="input input--block"
        v-if="createMode"
        type="password"
        v-model="passwordConfirm"
        :placeholder="t('login.passwordConfirm')"
      />

      <div v-if="recaptcha" id="recaptcha"></div>
      <input
        class="button button--block"
        type="submit"
        :value="createMode ? t('login.signup') : t('login.submit')"
      />

      <div id="signup">
        <p @click="toggleMode" v-if="signup">
          {{
            createMode ? t("login.loginInstead") : t("login.createAnAccount")
          }}
        </p>
      </div>
    </form>

    <div id="about">
      <p>
        Powered by
        <a href="https://github.com/versioneer-tech/package-r" target="_blank">
          <strong>packageR</strong>
        </a>
        {{ version }}, built on
        <a href="https://filebrowser.org" target="_blank">
          <img
            src="https://raw.githubusercontent.com/filebrowser/logo/master/banner.png"
            alt="Filebrowser"
            class="logo fb-logo"
          /> </a
        >, reassembled by
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
  signup,
  loginPage,
  version,
} from "@/utils/constants";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

// Define refs
const createMode = ref<boolean>(false);
const error = ref<string>("");
const username = ref<string>("");
const password = ref<string>("");
const passwordConfirm = ref<string>("");

const route = useRoute();
const router = useRouter();
const { t } = useI18n({});
// Define functions
const toggleMode = () => (createMode.value = !createMode.value);

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

  if (createMode.value) {
    if (password.value !== passwordConfirm.value) {
      error.value = t("login.passwordsDontMatch");
      return;
    }
  }

  try {
    if (createMode.value) {
      await auth.signup(username.value, password.value);
    }

    await auth.login(username.value, password.value, captcha);
    router.push({ path: redirect });
  } catch (e: any) {
    // console.error(e);
    if (e instanceof StatusError) {
      if (e.status === 409) {
        error.value = t("login.usernameTaken");
      } else if (e.status === 403) {
        error.value = t("login.wrongCredentials");
      } else {
        error.value = t("login.notPossible");
      }
    } else {
      $showError(e);
    }
  }
};

// Run hooks
onMounted(() => {
  if (!recaptcha) return;

  window.grecaptcha.ready(function () {
    window.grecaptcha.render("recaptcha", {
      sitekey: recaptchaKey,
    });
  });
});
</script>
