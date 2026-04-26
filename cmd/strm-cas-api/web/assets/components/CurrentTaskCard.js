import { stageText } from './ui.js'

function formatBytes(value) {
  const num = Number(value || 0)
  if (num < 1024) return `${num} B`
  if (num < 1024 * 1024) return `${(num / 1024).toFixed(1)} KB`
  if (num < 1024 * 1024 * 1024) return `${(num / 1024 / 1024).toFixed(1)} MB`
  return `${(num / 1024 / 1024 / 1024).toFixed(1)} GB`
}

function formatSpeed(value) {
  const num = Number(value || 0)
  if (!num) return '-'
  return `${formatBytes(num)}/s`
}

function formatETA(seconds) {
  const sec = Number(seconds || 0)
  if (!sec || sec < 0) return '-'
  if (sec < 60) return `${Math.round(sec)}s`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${Math.round(sec % 60)}s`
  return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`
}

function progressPercentOf(item) {
  const total = Number(item?.total_bytes || 0)
  const downloaded = Number(item?.downloaded_bytes || 0)
  if (!total) return 0
  return Math.max(0, Math.min(100, Math.round(downloaded / total * 100)))
}

export const CurrentTaskCard = {
  props: {
    current: { type: Object, default: null },
    activeCount: { type: Number, default: 0 },
    activeItems: { type: Array, default: () => [] },
    totalSpeedBytesPerSec: { type: Number, default: 0 },
  },
  methods: { stageText, formatBytes, formatSpeed, formatETA, progressPercentOf },
  template: `
    <div class="card">
      <div class="toolbar" style="justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap">
        <strong>活跃任务</strong>
        <span class="muted">活跃任务数：{{ activeCount || 0 }}</span>
        <span class="muted">当前总速度：{{ formatSpeed(totalSpeedBytesPerSec) }}</span>
      </div>
      <div v-if="activeItems.length" class="section">
        <div style="display:flex;flex-direction:column;gap:8px;margin-top:8px">
          <div v-for="(item, idx) in activeItems" :key="item.job?.strm_path || idx" class="card">
            <div class="mono">{{ item.job?.strm_path || '-' }}</div>
            <div class="row"><span>{{ stageText(item.stage) }}</span><span>{{ item.file_name || '' }}</span><span>{{ formatBytes(item.downloaded_bytes || 0) }}<template v-if="item.total_bytes"> / {{ formatBytes(item.total_bytes) }}</template></span></div>
            <div class="row"><span class="muted">速度：{{ formatSpeed(item.speed_bytes_per_sec) }}</span><span class="muted">ETA：{{ formatETA(item.eta_seconds) }}</span></div>
            <div v-if="item.total_bytes" class="section">
              <div class="progress-meta"><span>任务进度</span><strong>{{ progressPercentOf(item) }}%</strong></div>
              <div class="progress-bar"><div class="progress-inner" :style="{ width: progressPercentOf(item) + '%' }"></div></div>
            </div>
            <div class="muted">{{ item.message || '' }}</div>
          </div>
        </div>
      </div>
      <div v-else class="muted">当前没有活跃任务</div>
    </div>
  `,
}
