import type { RouteLocation } from "vue-router";
import { createRouter, createWebHistory } from "vue-router";
import Login from "@/views/Login.vue";
import Layout from "@/views/Layout.vue";
import Files from "@/views/Files.vue";
import Share from "@/views/Share.vue";
import Errors from "@/views/Errors.vue";
import { useAuthStore } from "@/stores/auth";
import { baseURL, name } from "@/utils/constants";
import i18n from "@/i18n";
import { recaptcha, loginPage } from "@/utils/constants";
import { login, validateLogin } from "@/utils/auth";

const titles = {
  Login: "sidebar.login",
  Share: "buttons.share",
  Files: "files.files",
  Forbidden: "errors.forbidden",
  NotFound: "errors.notFound",
  InternalServerError: "errors.internal",
};

const routes = [
  {
    path: "/login",
    name: "Login",
    component: Login,
  },
  {
    path: "/share",
    component: Layout,
    children: [
      {
        path: ":path*",
        name: "Share",
        component: Share,
      },
    ],
  },
  {
    path: "/files",
    component: Layout,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: ":path*",
        name: "Files",
        component: Files,
      },
    ],
  },
  {
    path: "/403",
    name: "Forbidden",
    component: Errors,
    props: {
      errorCode: 403,
      showHeader: true,
    },
  },
  {
    path: "/404",
    name: "NotFound",
    component: Errors,
    props: {
      errorCode: 404,
      showHeader: true,
    },
  },
  {
    path: "/500",
    name: "InternalServerError",
    component: Errors,
    props: {
      errorCode: 500,
      showHeader: true,
    },
  },
  {
    path: "/:catchAll(.*)*",
    redirect: (to: RouteLocation) => `/files${to.path}`,
  },
];

async function initAuth() {
  if (loginPage) {
    await validateLogin();
  } else {
    await login("", "", "");
  }

  if (recaptcha) {
    await new Promise<void>((resolve) => {
      const check = () => {
        if (typeof window.grecaptcha === "undefined") {
          setTimeout(check, 100);
        } else {
          resolve();
        }
      };

      check();
    });
  }
}

const router = createRouter({
  history: createWebHistory(baseURL),
  routes,
});

router.beforeResolve(async (to) => {
  const title = i18n.global.t(titles[to.name as keyof typeof titles]);
  document.title = title + " - " + name;

  const authStore = useAuthStore();
  const requiresAuth = to.matched.some((record) => record.meta.requiresAuth);

  if (requiresAuth && !authStore.isLoggedIn) {
    try {
      await initAuth();
    } catch (error) {
      console.error(error);
    }
  }

  if (requiresAuth && !authStore.isLoggedIn) {
    return {
      path: "/login",
      query: { redirect: to.fullPath },
    };
  }
});

export { router, router as default };
