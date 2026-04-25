export const BatchActionsBar = {
  props: {
    filters: { type: Object, required: true },
    loading: { type: Object, default: () => ({}) },
  },
  emits: ['start-current-filter', 'retry-current-filter'],
  computed: {
    hasFilter() {
      return !!(this.filters?.status || this.filters?.search)
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
      </div>
      <div class="toolbar section">
        <button @click="$emit('start-current-filter')" :disabled="!hasFilter || loading.start" :class="{ 'is-loading': loading.start }">按当前筛选开始任务</button>
        <button @click="$emit('retry-current-filter')" class="warning" :disabled="!hasFilter || loading.retryFailed" :class="{ 'is-loading': loading.retryFailed }">按当前筛选重试失败</button>
        <span class="muted">建议先筛选状态/关键词，再执行批量操作。</span>
      </div>
    </div>
  `,
}
