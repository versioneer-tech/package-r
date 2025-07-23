<template>
  <canvas ref="canvasRef" />
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import GeoTIFF from 'geotiff'

const props = defineProps({
  url: String
})

const canvasRef = ref(null)

const loadGeoTIFF = async () => {
  if (!props.url) return

  const tiff = await GeoTIFF.fromUrl(props.url)
  const image = await tiff.getImage()
  const width = image.getWidth()
  const height = image.getHeight()

  const canvas = canvasRef.value
  const ctx = canvas.getContext('2d')
  canvas.width = width
  canvas.height = height

const tileWidth = Math.min(512, image.getWidth());
const tileHeight = Math.min(512, image.getHeight());

const raster = await image.readRGB({
  window: [0, 0, tileWidth, tileHeight],
  width: tileWidth,
  height: tileHeight,
});

const imageData = ctx.createImageData(tileWidth, tileHeight);


  for (let i = 0; i < raster.length; i += 3) {
    const j = (i / 3) * 4
    imageData.data[j] = raster[i]        // Red
    imageData.data[j + 1] = raster[i+1]  // Green
    imageData.data[j + 2] = raster[i+2]  // Blue
    imageData.data[j + 3] = 255          // Alpha
  }

  ctx.putImageData(imageData, 0, 0)
}

onMounted(loadGeoTIFF)
watch(() => props.url, loadGeoTIFF)
</script>
