import { stageText } from './ui.js'

export const CurrentTaskCard = {
  props: { current: { type: Object, default: null } },
  methods: { stageText },
  template: `
    <div class="card">
      <div><strong>当前任务</strong></div>
      <template v-if="current">
        <div class="mono">{{ current.job?.strm_path || '' }}</div>
        <div class="row"><span>{{ stageText(current.stage) }}</span><span>{{ current.file_name || '' }}</span><span>{{ current.downloaded_bytes || 0 }}<template v-if="current.total_bytes"> / {{ current.total_bytes }}</template></span></div>
        <div class="muted">{{ current.message || '' }}</div>
      </template>
      <div v-else class="muted">暂无</div>
    </div>
  `,
}
