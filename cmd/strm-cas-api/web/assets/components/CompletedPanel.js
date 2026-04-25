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
    errorMessage: { type: String, default: '' },
  },
  emits: ['set-status', 'retry', 'detail', 'page-prev', 'page-next', 'page-jump'],
  computed: {
    emptyTitle() {
      return this.status ? '当前筛选下没有结果' : '暂无已完成任务'
    },
    emptyMessage() {
      return this.status ? '可以切换状态筛选，或先启动任务。' : '任务开始执行后，这里会展示最近完成的结果。'
    },
  },
  methods: { statusText },
  template: `
    <DataTableCard
      :total="completed.total || 0"
      :page="page || 1"
      :page-size="10"
      :loading="loading.completed"
      section-class="section"
      :empty-colspan="4"
      :empty-title="emptyTitle"
      :empty-message="emptyMessage"
      :error-message="errorMessage"
      :hide-pager-when-empty="true"
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
        <EmptyState v-if="!(completed.items || []).length" :colspan="4" :title="emptyTitle" :message="emptyMessage" />
        <tr v-for="item in (completed.items || [])" :key="(item.job?.strm_path || '') + (item.cas_path || '')">
          <td><span class="badge" :class="item.status || 'pending'">{{ statusText(item.status) }}</span></td>
          <td class="mono"><button class="link-button" @click="$emit('detail', item.job?.strm_path || '')">{{ item.job?.strm_path || '' }}</button></td>
          <td class="mono">{{ item.cas_path || '' }}</td>
          <td>{{ item.message || '' }} <button v-if="item.status === 'failed'" @click="$emit('retry', item.job?.strm_path || '')" class="warning" :disabled="loading.retryOne === (item.job?.strm_path || '')" :class="{ 'is-loading': loading.retryOne === (item.job?.strm_path || '') }">{{ loading.retryOne === (item.job?.strm_path || '') ? '重试中...' : '重试' }}</button></td>
        </tr>
      </template>
    </DataTableCard>
  `,
}
