<template>
  <div v-if="!checked" class="tiff-status">Checking...</div>
  <Errors v-else-if="!url || loadError" :errorCode="415" />
  <div v-else class="tiff-preview">
    <div class="tiff-image">
      <canvas ref="canvasEl" :class="{ hidden: !rendered }" />
    </div>
    <aside class="tiff-information">
      <template v-if="!rendered">
        <strong>{{ t("files.loading") }}</strong>
        <progress
          :value="downloadProgress"
          max="100"
          :aria-label="transferLabel"
        />
        <span data-testid="tiff-transfer" role="status">
          {{ transferLabel }}
        </span>
      </template>
      <template v-else>
        <strong>TIFF</strong>
        <dl>
          <template v-for="item in information" :key="item.label">
            <dt>{{ item.label }}</dt>
            <dd :data-testid="item.testId" :role="item.role">
              {{ item.value }}
            </dd>
          </template>
        </dl>
      </template>
    </aside>
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
const rendered = ref(false);
const canvasEl = ref(null);
const downloadedBytes = ref(0);
const rasterInfo = ref(null);
const fileStore = useFileStore();
const { t } = useI18n();

const totalSize = computed(() => filesize(fileStore.req?.size ?? 0));
const downloadedSize = computed(() => filesize(downloadedBytes.value));
const downloadProgress = computed(() => {
  const size = fileStore.req?.size ?? 0;
  return size > 0 ? Math.min(100, (downloadedBytes.value / size) * 100) : 0;
});
const transferLabel = computed(() =>
  t("files.previewTransfer", {
    downloaded: downloadedSize.value,
    total: totalSize.value,
  })
);
const information = computed(() => {
  if (!rasterInfo.value) return [];
  return [
    { label: t("files.tiffDimensions"), value: rasterInfo.value.dimensions },
    { label: t("files.tiffBands"), value: rasterInfo.value.bands },
    { label: t("files.tiffDataType"), value: rasterInfo.value.dataType },
    { label: t("files.tiffNoData"), value: rasterInfo.value.noData },
    { label: t("files.tiffMinimum"), value: rasterInfo.value.minimum },
    { label: t("files.tiffMaximum"), value: rasterInfo.value.maximum },
    { label: t("files.tiffMean"), value: rasterInfo.value.mean },
    { label: t("files.tiffStdDev"), value: rasterInfo.value.stdDev },
    {
      label: t("files.tiffTransfer"),
      value: `${downloadedSize.value} / ${totalSize.value}`,
      testId: "tiff-transfer",
      role: "status",
    },
  ];
});

function formatNumber(value) {
  if (value === null || value === undefined || value === "") return "—";
  const number = Number(value);
  if (!Number.isFinite(number)) return "—";
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(
    number
  );
}

function formatBandStatistic(metadata, name) {
  const values = metadata.map((band) => formatNumber(band?.[name]));
  return values.every((value) => value === "—") ? "—" : values.join(" / ");
}

function dataType(image, samples) {
  const prefixes = { 1: "UInt", 2: "Int", 3: "Float" };
  const types = Array.from({ length: samples }, (_, sample) => {
    const prefix = prefixes[image.getSampleFormat(sample)] ?? "Unknown";
    return `${prefix}${image.getBitsPerSample(sample)}`;
  });
  return [...new Set(types)].join(" / ");
}

async function renderTiff() {
  loadError.value = false;
  checked.value = false;
  rendered.value = false;
  downloadedBytes.value = 0;
  rasterInfo.value = null;

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
    const metadata = [];
    for (let sample = 0; sample < samples; sample++) {
      metadata.push(await image.getGDALMetadata(sample));
    }
    rasterInfo.value = {
      dimensions: `${fullWidth} × ${fullHeight}`,
      bands: String(samples),
      dataType: dataType(image, samples),
      noData: formatNumber(image.getGDALNoData()),
      minimum: formatBandStatistic(metadata, "STATISTICS_MINIMUM"),
      maximum: formatBandStatistic(metadata, "STATISTICS_MAXIMUM"),
      mean: formatBandStatistic(metadata, "STATISTICS_MEAN"),
      stdDev: formatBandStatistic(metadata, "STATISTICS_STDDEV"),
    };

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
    rendered.value = true;
    await nextTick();
    canvas.dataset.rendered = "true";
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
  display: flex;
  gap: 1.5rem;
  width: 100%;
  height: 100%;
  padding: 4rem 3.5rem 1rem;
}

.tiff-image {
  display: flex;
  flex: 1 1 auto;
  align-items: center;
  justify-content: center;
  min-width: 0;
}

.tiff-image canvas {
  display: block;
  max-width: 100%;
  max-height: 100%;
}

.tiff-image canvas.hidden {
  visibility: hidden;
}

.tiff-information {
  box-sizing: border-box;
  align-self: center;
  flex: 0 0 17rem;
  max-height: 100%;
  overflow: auto;
  padding: 1rem;
  border-radius: 0.35rem;
  color: #222;
  background: #fff;
  text-align: left;
}

.tiff-information strong {
  display: block;
  margin-bottom: 0.85rem;
}

.tiff-information progress {
  display: block;
  width: 100%;
  margin-bottom: 0.5rem;
}

.tiff-information span,
.tiff-information dl {
  margin: 0;
  font-size: 0.8rem;
  font-variant-numeric: tabular-nums;
}

.tiff-information dl {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 0.4rem 0.75rem;
}

.tiff-information dt {
  color: #666;
}

.tiff-information dd {
  margin: 0;
  overflow-wrap: anywhere;
}

@media (max-width: 737px) {
  .tiff-preview {
    gap: 0.75rem;
    padding: 4rem 1rem 1rem;
  }

  .tiff-information {
    flex-basis: 12rem;
  }
}
</style>
