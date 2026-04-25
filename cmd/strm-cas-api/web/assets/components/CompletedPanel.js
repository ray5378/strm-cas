import { PagerControl } from './PagerControl.js'
import { FilterTabs } from './FilterTabs.js'
import { statusText } from './ui.js'

export const CompletedPanel = {
  components: { PagerControl, FilterTabs },
  props: {
    completed: { type: Object, default: () => ({ total: 0, items: [] }) },
    status: { type: String, default: '' },
    page: { type: Number, default: 1 },
  },
  emits: ['set-status', 'retry', 'page-prev', 'page-next', 'page-jump'],
  methods: { statusText },
  template: `
    <div class="card section">
      <div class="toolbar records-toolbar">
        <strong>已完成任务</strong>
        <FilterTabs :model-value="status" @update:modelValue="$emit('set-status', $event)" />
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
