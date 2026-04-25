export const BatchActionsBar = {
  props: {
    filters: { type: Object, required: true },
    loading: { type: Object, default: () => ({}) },
    selectedCount: { type: Number, default: 0 },
  },
  emits: ['start-current-filter', 'retry-current-filter', 'start-selected', 'retry-selected', 'clear-selected'],
  computed: {
    hasFilter() {
      return !!(this.filters?.status || this.filters?.search)
    },
    hasSelection() {
      return this.selectedCount > 0
    },
    summary() {
      const parts = []
      if (this.filters?.status) parts.push(`状态=${this.filters.status}`)
      if (this.filters?.search) parts.push(`搜索=${this.filters.search}`)
      return parts.length ? parts.join('，') : '未设置筛选条件'
    },
  },
  template: `
    <div class="card section">
      <div class="toolbar">
        <strong>批量操作区</strong>
        <span class="muted">当前筛选：{{ summary }}</span>
        <span class="muted">已选中：{{ selectedCount }} 项</span>
      </div>
      <div class="toolbar section">
        <button @click="$emit('start-selected')" :disabled="!hasSelection || loading.start" :class="{ 'is-loading': loading.start }">开始选中项</button>
        <button @click="$emit('retry-selected')" class="warning" :disabled="!hasSelection || loading.retryFailed" :class="{ 'is-loading': loading.retryFailed }">重试选中失败项</button>
        <button @click="$emit('clear-selected')" class="secondary" :disabled="!hasSelection">清空选中</button>
      </div>
      <div class="toolbar section">
        <button @click="$emit('start-current-filter')" :disabled="!hasFilter || loading.start" :class="{ 'is-loading': loading.start }">按当前筛选开始任务</button>
        <button @click="$emit('retry-current-filter')" class="warning" :disabled="!hasFilter || loading.retryFailed" :class="{ 'is-loading': loading.retryFailed }">按当前筛选重试失败</button>
        <span class="muted">可先多选精确操作，也可直接按当前筛选批量处理。</span>
      </div>
    </div>
  `,
}
