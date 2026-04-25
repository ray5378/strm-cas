import { FilterTabs } from './FilterTabs.js'
import { DataTableCard } from './DataTableCard.js'
import { EmptyState } from './EmptyState.js'
import { statusText } from './ui.js'

export const CompletedPanel = {
  components: { FilterTabs, DataTableCard, EmptyState },
  props: {
    completed: { type: Object, default: () => ({ total: 0, items: [] }) },
    status: { type: String, default: '' },
    page: { type: Number, default: 1 },
    loading: { type: Object, default: () => ({}) },
  },
  emits: ['set-status', 'retry', 'page-prev', 'page-next', 'page-jump'],
  methods: { statusText },
  template: `
    <DataTableCard
      :total="completed.total || 0"
      :page="page || 1"
      :page-size="10"
      :loading="loading.completed"
      section-class="section"
      :empty-colspan="4"
      @prev="$emit('page-prev')"
      @next="$emit('page-next')"
      @jump="$emit('page-jump', $event)"
    >
      <template #header>
        <div class="toolbar records-toolbar">
          <strong>已完成任务</strong>
          <FilterTabs :model-value="status" @update:modelValue="$emit('set-status', $event)" />
        </div>
      </template>
      <template #thead><tr><th>状态</th><th>strm</th><th>cas</th><th>消息</th></tr></template>
      <template #rows>
        <EmptyState v-if="!(completed.items || []).length" :colspan="4" />
        <tr v-for="item in (completed.items || [])" :key="(item.job?.strm_path || '') + (item.cas_path || '')">
          <td><span class="badge" :class="item.status || 'pending'">{{ statusText(item.status) }}</span></td>
          <td class="mono">{{ item.job?.strm_path || '' }}</td>
          <td class="mono">{{ item.cas_path || '' }}</td>
          <td>{{ item.message || '' }} <button v-if="item.status === 'failed'" @click="$emit('retry', item.job?.strm_path || '')" class="warning" :disabled="loading.retryOne === (item.job?.strm_path || '')" :class="{ 'is-loading': loading.retryOne === (item.job?.strm_path || '') }">{{ loading.retryOne === (item.job?.strm_path || '') ? '重试中...' : '重试' }}</button></td>
        </tr>
      </template>
    </DataTableCard>
  `,
}
