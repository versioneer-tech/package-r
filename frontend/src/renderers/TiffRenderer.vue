<template>
  <div v-if="!checked" style="text-align: center; padding: 2em">
    Checking...
  </div>
  <Errors v-else-if="!url || loadError" :errorCode="415" />
  <div v-else style="padding: 1em">
    <canvas ref="canvasEl" style="max-width: 100%; border: 1px solid #ccc" />
  </div>
</template>

<script setup>
import { ref, onMounted } from "vue";
import { fromUrl } from "geotiff";
import Errors from "@/views/Errors.vue";

const props = defineProps({
  url: {
    type: String,
    required: false,
  },
});

const loadError = ref(false);
const checked = ref(false);
const canvasEl = ref(null);

async function renderTiff() {
  loadError.value = false;
  checked.value = false;

  if (!props.url) {
    loadError.value = true;
    checked.value = true;
    return;
  }

  try {
    const headCheck = await fetch(props.url, {
      method: "GET",
      headers: { Range: "bytes=0-0" },
    });
    if (!headCheck.ok)
      throw new Error(`HEAD check failed (${headCheck.status})`);
    checked.value = true;

    const tiff = await fromUrl(props.url);
    const image = await tiff.getImage();
    const fullWidth = image.getWidth();
    const fullHeight = image.getHeight();
    const samples = image.getSamplesPerPixel();
    const fileSize = image?.source?.fileSize ?? 0;
    const fileSizeMB = (fileSize / 1024 / 1024).toFixed(2);

    console.log(
      `[GeoTIFF] Full resolution: ${fullWidth}×${fullHeight}, ${samples} bands, ${fileSizeMB} MB`
    );

    const targetWidth = 768;
    const scaleFactor = targetWidth / fullWidth;
    const targetHeight = Math.round(fullHeight * scaleFactor);

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

    const rasters = await tiff.readRasters({
      width: targetWidth,
      height: targetHeight,
      interleave: false,
    });

    console.log(`[GeoTIFF] Loaded preview at ${targetWidth}×${targetHeight}`);
    console.log(`[GeoTIFF] Band 0 sample:`, rasters[0].slice(0, 10));

    const bandScales = rasters.map((band) => {
      const min = band.reduce((a, b) => Math.min(a, b));
      const max = band.reduce((a, b) => Math.max(a, b));
      return (val) => Math.round((255 * (val - min)) / (max - min || 1));
    });

    const imageData = ctx.createImageData(targetWidth, targetHeight);

    for (let i = 0; i < targetWidth * targetHeight; i++) {
      const r = rasters[0]?.[i] ?? 0;
      const g = rasters[1]?.[i] ?? r;
      const b = rasters[2]?.[i] ?? r;

      imageData.data.set(
        [
          bandScales[0](r),
          samples >= 3 ? bandScales[1](g) : bandScales[0](r),
          samples >= 3 ? bandScales[2](b) : bandScales[0](r),
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
