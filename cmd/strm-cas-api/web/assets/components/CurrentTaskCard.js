import { stageText } from './ui.js'

function formatBytes(value) {
  const num = Number(value || 0)
  if (num < 1024) return `${num} B`
  if (num < 1024 * 1024) return `${(num / 1024).toFixed(1)} KB`
  if (num < 1024 * 1024 * 1024) return `${(num / 1024 / 1024).toFixed(1)} MB`
  return `${(num / 1024 / 1024 / 1024).toFixed(1)} GB`
}

export const CurrentTaskCard = {
  props: { current: { type: Object, default: null } },
  methods: { stageText, formatBytes },
  computed: {
    progressPercent() {
      const total = Number(this.current?.total_bytes || 0)
      const downloaded = Number(this.current?.downloaded_bytes || 0)
      if (!total) return 0
      return Math.max(0, Math.min(100, Math.round(downloaded / total * 100)))
    },
  },
  template: `
    <div class="card">
      <div><strong>当前任务</strong></div>
      <template v-if="current">
        <div class="mono">{{ current.job?.strm_path || '' }}</div>
        <div class="row"><span>{{ stageText(current.stage) }}</span><span>{{ current.file_name || '' }}</span><span>{{ formatBytes(current.downloaded_bytes || 0) }}<template v-if="current.total_bytes"> / {{ formatBytes(current.total_bytes) }}</template></span></div>
        <div v-if="current.total_bytes" class="section">
          <div class="progress-meta"><span>当前下载进度</span><strong>{{ progressPercent }}%</strong></div>
          <div class="progress-bar"><div class="progress-inner" :style="{ width: progressPercent + '%' }"></div></div>
        </div>
        <div class="muted">{{ current.message || '' }}</div>
      </template>
      <div v-else class="muted">暂无</div>
    </div>
  `,
}
