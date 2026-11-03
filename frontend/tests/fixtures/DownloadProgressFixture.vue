<template>
<main class="p-8 space-y-6 max-w-xl">
    <h1 class="text-xl font-bold">下载进度组件验收</h1>
    <p>此页只操作隔离样本，不连接下载服务。</p>
    <div class="flex gap-4">
      <label>来源 <select v-model="task.source"><option>mikan</option><option>bt</option><option>rss</option><option>stream</option><option>manual</option></select></label>
      <label>下载类型 <select v-model="task.download_type"><option>torrent</option><option>stream</option><option>unknown</option></select></label>
      <label>状态 <select v-model="task.status"><option>downloading</option><option>pending</option><option>paused</option><option>failed</option><option>completed</option></select></label>
    </div>
    <label>实际百分比 <input v-model.number="task.progress" type="number" min="0" max="100"></label>
    <p>来源标签：{{ task.source }}；下载类型：{{ task.download_type }}</p>
    <div class="w-40"><DownloadTaskProgress :task="task" :t="t" /></div>
  </main>
</template>
<script setup>
import { reactive } from "vue"
import { useI18n } from "vue-i18n"
import DownloadTaskProgress from "../../src/components/Downloads/DownloadTaskProgress.vue"
const { t } = useI18n()
const task = reactive({ source: "mikan", download_type: "torrent", status: "downloading", progress: 42 })
</script>
