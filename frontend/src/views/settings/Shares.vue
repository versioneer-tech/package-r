<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="row" v-else-if="!layoutStore.loading">
    <div class="column">
      <div class="card">
        <div class="card-title">
          <h2>{{ t("settings.shareManagement") }}</h2>
        </div>

        <div class="card-content full" v-if="links.length > 0">
          <table>
            <tr>
              <th class="padded">{{ t("settings.path") }}</th>
              <th class="padded">{{ t("settings.shareDuration") }}</th>
              <th class="padded" v-if="authStore.user?.perm.admin">
                {{ t("settings.username") }}
              </th>
              <th class="padded">{{ t("settings.shareDescription") }}</th>
              <th class="padded">{{ $t("settings.shareGrant") }}</th>
              <th class="padded">{{ $t("settings.shareMode") }}</th>
              <th class="padded"></th>
              <th class="padded"></th>
            </tr>

            <tr v-for="link in links" :key="link.hash">
              <td>
                <a :href="buildLink(link)" target="_blank">{{ link.path }}</a>
              </td>
              <td>
                <template v-if="link.expire !== 0">{{
                  humanTime(link.expire)
                }}</template>
                <template v-else>{{ t("permanent") }}</template>
              </td>
              <td v-if="authStore.user?.perm.admin">{{ link.username }}</td>
              <td>
                {{ link.description }}
              </td>
              <td>{{ link.grant }}</td>
              <td>{{ link.mode }}</td>
              <td class="small">
                <button
                  class="action"
                  @click="deleteLink($event, link)"
                  :aria-label="t('buttons.delete')"
                  :title="t('buttons.delete')"
                >
                  <i class="material-icons">delete</i>
                </button>
              </td>
              <td class="small">
                <button
                  class="action copy-clipboard"
                  :aria-label="t('buttons.copyToClipboard')"
                  :title="t('buttons.copyToClipboard')"
                  @click="copyToClipboard(buildLink(link))"
                >
                  <i class="material-icons">content_paste</i>
                </button>
              </td>
            </tr>
          </table>
        </div>
        <h2 class="message" v-else>
          <i class="material-icons">sentiment_dissatisfied</i>
          <span>{{ t("files.lonely") }}</span>
        </h2>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { share as api, users } from "@/api";
import dayjs from "dayjs";
import Errors from "@/views/Errors.vue";
import { inject, ref, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { StatusError } from "@/api/utils";
import { copy } from "@/utils/clipboard";

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();

const layoutStore = useLayoutStore();
const authStore = useAuthStore();

const error = ref<StatusError | null>(null);
const links = ref<Share[]>([]);

onMounted(async () => {
  layoutStore.loading = true;

  try {
    let newLinks = await api.list();
    if (authStore.user?.perm.admin) {
      let userMap = new Map<number, string>();
      for (let user of await users.getAll())
        userMap.set(user.id, user.username);
      for (let link of newLinks) {
        if (link.userID && userMap.has(link.userID))
          link.username = userMap.get(link.userID);
      }
    }
    links.value = newLinks;
  } catch (err) {
    if (err instanceof Error) {
      error.value = err;
    }
  } finally {
    layoutStore.loading = false;
  }
});

const copyToClipboard = (text: string) => {
  copy(text).then(
    () => {
      // clipboard successfully set
      $showSuccess(t("success.linkCopied"));
    },
    () => {
      // clipboard write failed
    }
  );
};

const deleteLink = async (event: Event, link: any) => {
  event.preventDefault();

  layoutStore.showHover({
    prompt: "share-delete",
    confirm: () => {
      layoutStore.closeHovers();

      try {
        api.remove(link.hash);
        links.value = links.value.filter((item) => item.hash !== link.hash);
        $showSuccess(t("settings.shareDeleted"));
      } catch (err) {
        if (err instanceof Error) {
          $showError(err);
        }
      }
    },
  });
};
const humanTime = (time: number) => {
  return dayjs(time * 1000).isAfter("1.1.2000")
    ? dayjs(time * 1000).fromNow()
    : "";
};

const buildLink = (share: Share) => {
  return api.getShareURL(share);
};
</script>

<style scoped>
.padded {
  padding-right: 30px;
}
</style>
