<template>
  <ModalsContainer />
</template>

<script setup lang="ts">
import { watch } from "vue";
import { ModalsContainer, useModal } from "vue-final-modal";
import { useLayoutStore } from "@/stores/layout";

import BaseModal from "./BaseModal.vue";
import Help from "./Help.vue";
import Info from "./Info.vue";
import Delete from "./Delete.vue";
import Download from "./Download.vue";
import Rename from "./Rename.vue";
import Move from "./Move.vue";
import Copy from "./Copy.vue";
import NewFile from "./NewFile.vue";
import NewDir from "./NewDir.vue";
import Replace from "./Replace.vue";
import ReplaceRename from "./ReplaceRename.vue";
import Share from "./Share.vue";
import Upload from "./Upload.vue";
import DiscardEditorChanges from "./DiscardEditorChanges.vue";

const layoutStore = useLayoutStore();

const components = new Map<string, any>([
  ["info", Info],
  ["help", Help],
  ["delete", Delete],
  ["rename", Rename],
  ["move", Move],
  ["copy", Copy],
  ["newFile", NewFile],
  ["newDir", NewDir],
  ["download", Download],
  ["replace", Replace],
  ["replace-rename", ReplaceRename],
  ["share", Share],
  ["upload", Upload],
  ["discardEditorChanges", DiscardEditorChanges],
]);

watch(
  () => layoutStore.prompts.length,
  (length, previousLength) => {
    if (length <= previousLength) return;

    const prompt = layoutStore.prompts[length - 1];
    const modal = components.get(prompt.prompt);
    if (!modal) return;

    const { open, close } = useModal({
      component: BaseModal,
      slots: {
        default: modal,
      },
    });

    layoutStore.setCloseOnCurrentPrompt(close);
    open();
  }
);

window.addEventListener("keydown", (event) => {
  if (!layoutStore.currentPrompt) return;

  if (event.key === "Escape") {
    event.stopImmediatePropagation();
    layoutStore.closeHovers();
  }
});
</script>
