<template>
  <div v-if="!checked" class="tiff-status">Checking...</div>
  <Errors v-else-if="!url || loadError" :errorCode="415" />
  <div v-else class="tiff-preview">
    <canvas ref="canvasEl" />
    <div
      class="tiff-transfer"
      data-testid="tiff-transfer"
      role="status"
      :aria-label="
        t('files.previewTransfer', {
          downloaded: downloadedSize,
          total: totalSize,
        })
      "
    >
      {{ downloadedSize }} / {{ totalSize }}
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref } from "vue";
import { fromCustomClient } from "geotiff";
import { filesize } from "filesize";
import Errors from "@/views/Errors.vue";
import { useFileStore } from "@/stores/file";
import { PartialContentClient } from "@/utils/partialContentClient";
import { useI18n } from "vue-i18n";

const props = defineProps({
  url: {
    type: String,
    required: false,
  },
});

const loadError = ref(false);
const checked = ref(false);
const canvasEl = ref(null);
const downloadedBytes = ref(0);
const fileStore = useFileStore();
const { t } = useI18n();

const totalSize = computed(() => filesize(fileStore.req?.size ?? 0));
const downloadedSize = computed(() => filesize(downloadedBytes.value));

async function renderTiff() {
  loadError.value = false;
  checked.value = false;
  downloadedBytes.value = 0;

  const objectSize = fileStore.req?.size ?? 0;
  if (!props.url || objectSize <= 0) {
    loadError.value = true;
    checked.value = true;
    return;
  }

  try {
    checked.value = true;
    await nextTick();

    const tiff = await fromCustomClient(
      new PartialContentClient(props.url, objectSize, (bytes) => {
        downloadedBytes.value += bytes;
      })
    );
    const image = await tiff.getImage();
    const fullWidth = image.getWidth();
    const fullHeight = image.getHeight();
    const samples = image.getSamplesPerPixel();
    const fileSize = image?.source?.fileSize ?? 0;
    const fileSizeMB = (fileSize / 1024 / 1024).toFixed(2);

    const targetWidth = Math.min(768, fullWidth);
    const scaleFactor = targetWidth / fullWidth;
    const targetHeight = Math.round(fullHeight * scaleFactor);

    let previewImage = image;
    const imageCount = await tiff.getImageCount();
    for (let index = 1; index < imageCount; index++) {
      const candidate = await tiff.getImage(index);
      if (
        candidate.getWidth() >= targetWidth &&
        candidate.getWidth() < previewImage.getWidth()
      ) {
        previewImage = candidate;
      }
    }

    const canvas = canvasEl.value;
    canvas.width = targetWidth;
    canvas.height = targetHeight;
    const ctx = canvas.getContext("2d");

    ctx.fillStyle = "#f5f5f5";
    ctx.fillRect(0, 0, targetWidth, targetHeight);
    ctx.fillStyle = "#333";
    ctx.font = "16px sans-serif";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";

    const lines = [
      `Loading...`,
      ``,
      `${fullWidth}×${fullHeight} px`,
      `${samples} band(s)`,
      `${fileSizeMB} MB`,
    ];

    lines.forEach((text, i) => {
      ctx.fillText(text, targetWidth / 2, targetHeight / 2 - 40 + i * 20);
    });

    const rgb = await previewImage.readRGB({
      width: targetWidth,
      height: targetHeight,
      interleave: true,
    });
    const maxSampleValue =
      rgb.BYTES_PER_ELEMENT === 1
        ? 255
        : 2 ** previewImage.getBitsPerSample(0) - 1;
    const toByte = (value) => Math.round((255 * value) / maxSampleValue);

    const imageData = ctx.createImageData(targetWidth, targetHeight);

    for (let i = 0; i < targetWidth * targetHeight; i++) {
      imageData.data.set(
        [
          toByte(rgb[i * 3] ?? 0),
          toByte(rgb[i * 3 + 1] ?? 0),
          toByte(rgb[i * 3 + 2] ?? 0),
          255,
        ],
        i * 4
      );
    }

    ctx.putImageData(imageData, 0, 0);
  } catch (err) {
    console.error("[GeoTIFF] Rendering failed:", err);
    loadError.value = true;
    checked.value = true;
  }
}

onMounted(renderTiff);
</script>

<style scoped>
.tiff-status {
  box-sizing: border-box;
  height: 100%;
  padding-top: 50vh;
  color: rgba(255, 255, 255, 0.75);
  text-align: center;
}

.tiff-preview {
  box-sizing: border-box;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  padding: 4rem 3.5rem 1rem;
}

.tiff-preview canvas {
  display: block;
  max-width: 100%;
  max-height: 100%;
}

.tiff-transfer {
  position: absolute;
  right: 1rem;
  bottom: 1rem;
  padding: 0.35rem 0.55rem;
  border-radius: 0.35rem;
  color: rgba(255, 255, 255, 0.85);
  background: rgba(0, 0, 0, 0.65);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
  line-height: 1;
  pointer-events: none;
}
</style>
