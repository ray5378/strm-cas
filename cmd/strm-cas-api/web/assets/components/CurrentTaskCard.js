import { stageText } from './ui.js'

function formatBytes(value) {
  const num = Number(value || 0)
  if (num < 1024) return `${num} B`
  if (num < 1024 * 1024) return `${(num / 1024).toFixed(1)} KB`
  if (num < 1024 * 1024 * 1024) return `${(num / 1024 / 1024).toFixed(1)} MB`
  return `${(num / 1024 / 1024 / 1024).toFixed(1)} GB`
}

export const CurrentTaskCard = {
  props: {
    current: { type: Object, default: null },
    activeCount: { type: Number, default: 0 },
    activeItems: { type: Array, default: () => [] },
  },
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
      <div class="toolbar" style="justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap">
        <strong>当前任务</strong>
        <span class="muted">活跃任务数：{{ activeCount || 0 }}</span>
      </div>
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
      <div v-if="activeItems.length" class="section">
        <strong>活跃任务列表</strong>
        <div style="display:flex;flex-direction:column;gap:8px;margin-top:8px">
          <div v-for="(item, idx) in activeItems" :key="idx" class="card">
            <div class="mono">{{ item.job?.strm_path || '-' }}</div>
            <div class="row"><span>{{ stageText(item.stage) }}</span><span>{{ item.file_name || '' }}</span><span>{{ formatBytes(item.downloaded_bytes || 0) }}<template v-if="item.total_bytes"> / {{ formatBytes(item.total_bytes) }}</template></span></div>
          </div>
        </div>
      </div>
    </div>
  `,
}
