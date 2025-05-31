<template>
  <div class="card floating" id="share">
    <div class="card-title">
      <h2>{{ $t("buttons.share") }}</h2>
    </div>

    <template v-if="listing">
      <div class="card-content">
        <table>
          <tr>
            <th>#</th>
            <th>{{ $t("settings.shareDuration") }}</th>
            <th>{{ $t("settings.shareDescription") }}</th>
            <th>{{ $t("settings.shareGrant") }}</th>
            <th>{{ $t("settings.shareMode") }}</th>
            <th></th>
            <th></th>
          </tr>

          <tr v-for="link in links" :key="link.hash">
            <td>{{ link.hash }}</td>
            <td>
              <template v-if="link.expire !== 0">{{
                humanTime(link.expire)
              }}</template>
              <template v-else>{{ $t("permanent") }}</template>
            </td>
            <td>{{ link.description }}</td>
            <td>{{ link.grant }}</td>
            <td>{{ link.mode }}</td>
            <td class="small">
              <button
                class="action copy-clipboard"
                :aria-label="$t('buttons.copyToClipboard')"
                :title="$t('buttons.copyToClipboard')"
                @click="copyToClipboard(buildLink(link))"
              >
                <i class="material-icons">content_paste</i>
              </button>
            </td>
            <td class="small" v-if="hasDownloadLink()">
              <button
                class="action copy-clipboard"
                :aria-label="$t('buttons.copyDownloadLinkToClipboard')"
                :title="$t('buttons.copyDownloadLinkToClipboard')"
                @click="copyToClipboard(buildDownloadLink(link))"
              >
                <i class="material-icons">content_paste_go</i>
              </button>
            </td>
            <td class="small">
              <button
                class="action"
                @click="deleteLink($event, link)"
                :aria-label="$t('buttons.delete')"
                :title="$t('buttons.delete')"
              >
                <i class="material-icons">delete</i>
              </button>
            </td>
          </tr>
        </table>
      </div>

      <div class="card-action">
        <button
          class="button button--flat button--grey"
          @click="closeHovers"
          :aria-label="$t('buttons.close')"
          :title="$t('buttons.close')"
          tabindex="2"
        >
          {{ $t("buttons.close") }}
        </button>
        <button
          id="focus-prompt"
          class="button button--flat button--blue"
          @click="() => switchListing()"
          :aria-label="$t('buttons.new')"
          :title="$t('buttons.new')"
          tabindex="1"
        >
          {{ $t("buttons.new") }}
        </button>
      </div>
    </template>

    <template v-else>
      <div class="card-content">
        <p>{{ $t("settings.shareDuration") }}</p>
        <div class="input-group input">
          <vue-number-input
            center
            controls
            size="small"
            :max="2147483647"
            :min="0"
            @keyup.enter="submit"
            v-model="time"
            tabindex="1"
          />
          <select
            class="right"
            v-model="unit"
            :aria-label="$t('time.unit')"
            tabindex="2"
          >
            <option value="seconds">{{ $t("time.seconds") }}</option>
            <option value="minutes">{{ $t("time.minutes") }}</option>
            <option value="hours">{{ $t("time.hours") }}</option>
            <option value="days">{{ $t("time.days") }}</option>
          </select>
        </div>
        <p>{{ $t("prompts.optionalPassword") }}</p>
        <input
          class="input input--block"
          type="password"
          v-model.trim="password"
          tabindex="3"
        />
        <p>{{ $t("settings.shareDescription") }}</p>
        <input
          class="input input--block"
          v-model.trim="description"
          tabindex="4"
        />
        <p>{{ $t("settings.shareGrant") }}</p>
        <select class="input input--block" v-model="grant" tabindex="5">
          <option value="">-</option>
          <option v-for="group in groups" :key="group" :value="group">
            {{ group }}
          </option>
        </select>
        <p>{{ $t("settings.shareMode") }}</p>
        <select class="input input--block" v-model="mode" tabindex="5">
          <option value="">default</option>
          <option
            value="indexed"
            v-if="!this.url.startsWith('/files/.packages')"
          >
            indexed
          </option>
        </select>
      </div>

      <div class="card-action">
        <button
          class="button button--flat button--grey"
          @click="() => switchListing()"
          :aria-label="$t('buttons.cancel')"
          :title="$t('buttons.cancel')"
          tabindex="6"
        >
          {{ $t("buttons.cancel") }}
        </button>
        <button
          id="focus-prompt"
          class="button button--flat button--blue"
          @click="submit"
          :aria-label="$t('buttons.share')"
          :title="$t('buttons.share')"
          tabindex="7"
        >
          {{ $t("buttons.share") }}
        </button>
      </div>
    </template>
  </div>
</template>

<script>
import { mapActions, mapState } from "pinia";
import { useFileStore } from "@/stores/file";
import { share as api, pub as pub_api } from "@/api";
import dayjs from "dayjs";
import { useLayoutStore } from "@/stores/layout";
import { copy } from "@/utils/clipboard";
import { baseURL } from "@/utils/constants";
import { useAuthStore } from "@/stores/auth";

export default {
  name: "share",
  data: function () {
    return {
      time: 0,
      unit: "hours",
      links: [],
      clip: null,
      password: "",
      description: "",
      grant: "",
      mode: "",
      listing: true,
      groups: [],
    };
  },
  inject: ["$showError", "$showSuccess"],
  computed: {
    ...mapState(useFileStore, [
      "req",
      "selected",
      "selectedCount",
      "isListing",
    ]),
    url() {
      if (!this.isListing) {
        return this.$route.path;
      }

      if (this.selectedCount === 0 || this.selectedCount > 1) {
        return;
      }

      return this.req.items[this.selected[0]].url;
    },
  },
  async beforeMount() {
    try {
      const links = await api.get(this.url);
      this.links = links;
      this.sort();

      if (this.links.length === 0) {
        this.listing = false;
      }

      this.fetchGroups();
    } catch (e) {
      this.$showError(e);
    }
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
    copyToClipboard: function (text) {
      copy(text).then(
        () => {
          this.$showSuccess(this.$t("success.linkCopied"));
        },
        () => {
          // clipboard write failed
        }
      );
    },
    submit: async function () {
      try {
        let res = null;

        if (!this.time) {
          res = await api.create(
            this.url,
            this.password,
            this.description,
            this.grant,
            this.mode
          );
        } else {
          res = await api.create(
            this.url,
            this.password,
            this.description,
            this.grant,
            this.mode,
            this.time,
            this.unit
          );
        }

        this.links.push(res);
        this.sort();

        this.time = 0;
        this.unit = "hours";
        this.password = "";
        this.description = "";
        this.grant = "";
        this.mode = "";

        this.listing = true;
      } catch (e) {
        this.$showError(e);
      }
    },
    deleteLink: async function (event, link) {
      event.preventDefault();
      try {
        await api.remove(link.hash);
        this.links = this.links.filter((item) => item.hash !== link.hash);

        if (this.links.length === 0) {
          this.listing = false;
        }
      } catch (e) {
        this.$showError(e);
      }
    },
    humanTime(time) {
      return dayjs(time * 1000).isAfter("1.1.2000")
        ? dayjs(time * 1000).fromNow()
        : "";
    },
    buildLink(share) {
      return api.getShareURL(share);
    },
    hasDownloadLink() {
      return (
        this.selected.length === 1 && !this.req.items[this.selected[0]].isDir
      );
    },
    buildDownloadLink(share) {
      return pub_api.getDownloadURL(share);
    },
    sort() {
      this.links = this.links.sort((a, b) => {
        if (a.expire === 0) return -1;
        if (b.expire === 0) return 1;
        return new Date(a.expire) - new Date(b.expire);
      });
    },
    switchListing() {
      if (this.links.length === 0 && !this.listing) {
        this.closeHovers();
      }

      this.listing = !this.listing;
    },
    fetchGroups() {
      const ssl = window.location.protocol === "https:";
      const protocol = ssl ? "wss:" : "ws:";
      const authStore = useAuthStore();
      const url = `${protocol}//${window.location.host}${baseURL}/api/command/?auth=${authStore.jwt}`;

      console.log("Fetching groups...");
      this.groups = []; // Clear groups array before fetching new ones
      const ws = new WebSocket(url);

      ws.onopen = () => {
        console.log("WebSocket connected for fetching groups.");
        ws.send("get-groups");
      };

      ws.onmessage = (event) => {
        console.log("WebSocket message received:", event.data);
        try {
          const group = event.data;
          if (group && !this.groups.includes(group)) {
            this.groups.push(group);
          }
        } catch (error) {
          console.error("Error parsing WebSocket message:", error);
        }
      };

      ws.onerror = (error) => {
        console.error("WebSocket error during fetch:", error);
        this.$showError(error);
      };

      ws.onclose = () => {
        console.log("WebSocket closed after fetching groups.");
      };
    },
  },
};
</script>
