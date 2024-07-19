import dayjs from "dayjs";
import { createI18n } from "vue-i18n";

import("dayjs/locale/de");
import("dayjs/locale/en");

// All i18n resources specified in the plugin `include` option can be loaded
// at once using the import syntax
import messages from "@intlify/unplugin-vue-i18n/messages";

export function detectLocale() {
  // locale is an RFC 5646 language tag
  // https://developer.mozilla.org/en-US/docs/Web/API/Navigator/language
  let locale = navigator.language.toLowerCase();
  switch (true) {
    case /^en\b/.test(locale):
      locale = "en";
      break;
    case /^de\b/.test(locale):
      locale = "de";
      break;
    default:
      locale = "en";
  }

  return locale;
}

// TODO: was this really necessary?
// function removeEmpty(obj: Record<string, any>): void {
//   Object.keys(obj)
//     .filter((k) => obj[k] !== null && obj[k] !== undefined && obj[k] !== "") // Remove undef. and null and empty.string.
//     .reduce(
//       (newObj, k) =>
//         typeof obj[k] === "object"
//           ? Object.assign(newObj, { [k]: removeEmpty(obj[k]) }) // Recurse.
//           : Object.assign(newObj, { [k]: obj[k] }), // Copy value.
//       {}
//     );
// }

export const rtlLanguages = ["he", "ar"];

export const i18n = createI18n({
  locale: detectLocale(),
  fallbackLocale: "en",
  messages,
  // expose i18n.global for outside components
  legacy: true,
});

export const isRtl = (locale?: string) => {
  // see below
  // @ts-ignore
  return rtlLanguages.includes(locale || i18n.global.locale.value);
};

export function setLocale(locale: string) {
  dayjs.locale(locale);
  // according to doc u only need .value if legacy: false but they lied
  // https://vue-i18n.intlify.dev/guide/essentials/scope.html#local-scope-1
  //@ts-ignore
  i18n.global.locale.value = locale;
}

export function setHtmlLocale(locale: string) {
  const html = document.documentElement;
  html.lang = locale;
  if (isRtl(locale)) html.dir = "rtl";
  else html.dir = "ltr";
}

export default i18n;
