<template>
  <select name="selectLanguage" :value="locale" @change="change">
    <option v-for="(language, value) in locales" :key="value" :value="value">
      {{ language }}
    </option>
  </select>
</template>

<script setup lang="ts">
defineProps<{ locale: string }>();

const emit = defineEmits<{
  "update:locale": [value: string];
}>();

const locales = {
  ar: "العربية",
  ca: "Català",
  de: "Deutsch",
  el: "Ελληνικά",
  en: "English",
  es: "Español",
  fr: "Français",
  he: "עברית",
  hu: "Magyar",
  is: "Íslenska",
  it: "Italiano",
  ja: "日本語",
  ko: "한국어",
  "nl-be": "Nederlands (België)",
  pl: "Polski",
  "pt-br": "Português (Brasil)",
  pt: "Português",
  ro: "Română",
  ru: "Русский",
  sk: "Slovenčina",
  "sv-se": "Svenska",
  tr: "Türkçe",
  uk: "Українська",
  "zh-cn": "中文 (简体)",
  "zh-tw": "中文 (繁體)",
};

const change = (event: Event) => {
  emit("update:locale", (event.target as HTMLSelectElement).value);
};
</script>
