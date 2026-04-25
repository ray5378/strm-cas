import { PagerControl } from './PagerControl.js'
import { statusText } from './ui.js'

const FILTERS = [['','全部'], ['pending','未处理'], ['done','已完成'], ['failed','失败'], ['exception','异常'], ['skipped','已跳过']]

export const CompletedPanel = {
  components: { PagerControl },
  props: {
    completed: { type: Object, default: () => ({ total: 0, items: [] }) },
    status: { type: String, default: '' },
    page: { type: Number, default: 1 },
  },
  emits: ['set-status', 'retry', 'page-prev', 'page-next', 'page-jump'],
  methods: { statusText },
  template: `
    <div class="card section">
      <div class="toolbar">
        <strong>已完成任务</strong>
        <button v-for="([value, label], idx) in ${JSON.stringify(FILTERS)}" :key="idx" @click="$emit('set-status', value)" :class="{ active: status === value }">{{ label }}</button>
      </div>
      <table>
        <thead><tr><th>状态</th><th>strm</th><th>cas</th><th>消息</th></tr></thead>
        <tbody>
          <tr v-if="!(completed.items || []).length"><td colspan="4" class="muted">无数据</td></tr>
          <tr v-for="item in (completed.items || [])" :key="(item.job?.strm_path || '') + (item.cas_path || '')">
            <td><span class="badge" :class="item.status || 'pending'">{{ statusText(item.status) }}</span></td>
            <td class="mono">{{ item.job?.strm_path || '' }}</td>
            <td class="mono">{{ item.cas_path || '' }}</td>
            <td>{{ item.message || '' }} <button v-if="item.status === 'failed'" @click="$emit('retry', item.job?.strm_path || '')" class="warning">重试</button></td>
          </tr>
        </tbody>
      </table>
      <PagerControl :total="completed.total || 0" :page="page || 1" :page-size="10" @prev="$emit('page-prev')" @next="$emit('page-next')" @jump="$emit('page-jump', $event)" />
    </div>
  `,
}
