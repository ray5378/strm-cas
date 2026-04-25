import { FilterTabs } from './FilterTabs.js'
import { DataTableCard } from './DataTableCard.js'
import { EmptyState } from './EmptyState.js'
import { statusText } from './ui.js'

export const RecordsPanel = {
  components: { FilterTabs, DataTableCard, EmptyState },
  props: {
    records: { type: Object, default: () => ({ total: 0, items: [] }) },
    filters: { type: Object, required: true },
    loading: { type: Object, default: () => ({}) },
    errorMessage: { type: String, default: '' },
    selectedPaths: { type: Array, default: () => [] },
  },
  emits: ['set-status', 'apply-search', 'detail', 'retry', 'page-prev', 'page-next', 'page-jump', 'toggle-selected', 'toggle-select-all'],
  data() {
    return { searchValue: this.filters.search || '' }
  },
  computed: {
    emptyTitle() {
      return this.filters.search || this.filters.status ? '没有匹配结果' : '暂无数据库记录'
    },
    emptyMessage() {
      if (this.filters.search || this.filters.status) return '试试调整筛选条件或先执行扫描。'
      return '先点击“扫描 /strm”，把当前任务同步进数据库。'
    },
    currentPagePaths() {
      return (this.records.items || []).map(item => item.strm_path)
    },
    allCurrentSelected() {
      return this.currentPagePaths.length > 0 && this.currentPagePaths.every(path => this.selectedPaths.includes(path))
    },
  },
  watch: {
    'filters.search'(v) { this.searchValue = v || '' },
  },
  methods: { statusText },
  template: `
    <DataTableCard
      :total="records.total || 0"
      :page="filters.page || 1"
      :page-size="filters.page_size || 10"
      :loading="loading.records"
      :empty-colspan="6"
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
          <strong>数据库记录</strong>
          <FilterTabs :model-value="filters.status" @update:modelValue="$emit('set-status', $event)" />
          <div class="row grow">
            <input v-model="searchValue" placeholder="搜索路径 / URL / 错误" class="grow-input" />
            <button @click="$emit('apply-search', searchValue)" :disabled="loading.records">筛选</button>
          </div>
        </div>
      </template>
      <template #thead>
        <tr><th><input type="checkbox" :checked="allCurrentSelected" @change="$emit('toggle-select-all', currentPagePaths)" /></th><th>状态</th><th>strm</th><th>cas</th><th>最后结果</th><th></th></tr>
      </template>
      <template #rows>
        <EmptyState v-if="!(records.items || []).length" :colspan="6" :title="emptyTitle" :message="emptyMessage" />
        <tr v-for="item in (records.items || [])" :key="item.strm_path">
          <td><input type="checkbox" :checked="selectedPaths.includes(item.strm_path)" @change="$emit('toggle-selected', item.strm_path)" /></td>
          <td><span class="badge" :class="item.status || 'pending'">{{ statusText(item.status) }}</span></td>
          <td class="mono">{{ item.strm_path }}</td>
          <td class="mono">{{ item.cas_path || '' }}</td>
          <td>{{ item.last_message || '' }}</td>
          <td>
            <button @click="$emit('detail', item.strm_path)" :disabled="loading.detail">详情</button>
            <button v-if="item.status === 'failed'" @click="$emit('retry', item.strm_path)" class="warning" :disabled="loading.retryOne === item.strm_path" :class="{ 'is-loading': loading.retryOne === item.strm_path }">{{ loading.retryOne === item.strm_path ? '重试中...' : '重试' }}</button>
          </td>
        </tr>
      </template>
    </DataTableCard>
  `,
}
