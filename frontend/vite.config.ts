import path from "node:path";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import legacy from "@vitejs/plugin-legacy";
import { compression } from "vite-plugin-compression2";

const playwrightSTACBrowserURL = process.env.PLAYWRIGHT_STAC_BROWSER_URL || "";

const plugins = [
  {
    name: "development-config",
    transformIndexHtml(html: string) {
      return html.replace(
        "__PLAYWRIGHT_STAC_BROWSER_URL__",
        playwrightSTACBrowserURL
      );
    },
  },
  vue(),
  VueI18nPlugin({
    include: [path.resolve(import.meta.dirname, "./src/i18n/**/*.json")],
  }),
  legacy({
    // defaults already drop IE support
    targets: ["defaults"],
  }),
  compression({ include: /\.js$/i, deleteOriginalAssets: true }),
];

const resolve = {
  alias: {
    // vue: "@vue/compat",
    "@/": `${path.resolve(import.meta.dirname, "src")}/`,
  },
};

const backendURL =
  process.env.PACKAGE_R_BACKEND_URL ||
  `http://127.0.0.1:${process.env.PACKAGE_R_PORT || "8888"}`;
const backendWsURL = backendURL.replace(/^http/, "ws");

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  if (command === "serve") {
    return {
      plugins,
      resolve,
      server: {
        proxy: {
          "/api/command": {
            target: backendWsURL,
            ws: true,
          },
          "/api": backendURL,
        },
      },
    };
  } else {
    // command === 'build'
    return {
      plugins,
      resolve,
      base: "",
      build: {
        rollupOptions: {
          input: {
            index: path.resolve(import.meta.dirname, "./public/index.html"),
          },
          output: {
            manualChunks: (id) => {
              // bundle dayjs files in a single chunk
              // this avoids having small files for each locale
              if (id.includes("dayjs/")) {
                return "dayjs";
                // bundle i18n in a separate chunk
              } else if (id.includes("i18n/")) {
                return "i18n";
              }
            },
          },
        },
      },
      experimental: {
        renderBuiltUrl(filename, { hostType }) {
          if (hostType === "js") {
            return { runtime: `window.__prependStaticUrl("${filename}")` };
          } else if (hostType === "html") {
            return `[{[ .StaticURL ]}]/${filename}`;
          } else {
            return { relative: true };
          }
        },
      },
    };
  }
});
