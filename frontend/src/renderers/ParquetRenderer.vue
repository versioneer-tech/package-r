<template>
  <table v-if="rows.length">
    <thead><tr><th v-for="col in columns" :key="col">{{ col }}</th></tr></thead>
    <tbody>
      <tr v-for="(row, i) in rows" :key="i">
        <td v-for="col in columns" :key="col">{{ row[col] }}</td>
      </tr>
    </tbody>
  </table>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import parquet from 'parquet-wasm';
import { useFileStore } from '@/stores/file';

const rows = ref<any[]>([]);
const columns = ref<string[]>([]);
const fileStore = useFileStore();

onMounted(async () => {
  if (fileStore.req?.presignedURL) {
    const res = await fetch(fileStore.req.presignedURL);
    const buf = await res.arrayBuffer();
    const reader = await parquet.ParquetReader.openBuffer(buf);
    const cursor = reader.getCursor();
    let rec;
    while ((rec = await cursor.next())) rows.value.push(rec);
    await reader.close();
    columns.value = rows.value.length ? Object.keys(rows.value[0]) : [];
  }
});
</script>
