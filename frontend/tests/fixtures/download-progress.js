// 仅由 Vite 开发服务器提供，不进入产品构建；直接验收正式组件。
import { createApp } from 'vue'
import { createI18n } from 'vue-i18n'
import DownloadProgressFixture from './DownloadProgressFixture.vue'
import '../../src/assets/tailwind.css'

const messages = { 'zh-CN': { status: { completed: '已完成', paused: '已暂停', failed: '已失败', superseded: '已替代' }, pages: { downloads: {
  stageQueued: '排队', stageMetadata: '获取元数据', stageDownloading: '下载中', stageCompleted: '完成', stageStalled: '停滞', stageSeekingAlternative: '寻找替代源',
} } } }
const i18n = createI18n({ legacy: false, locale: 'zh-CN', messages })
createApp(DownloadProgressFixture).use(i18n).mount("#app")
