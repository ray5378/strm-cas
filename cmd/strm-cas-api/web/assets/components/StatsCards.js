export const StatsCards = {
  props: {
    stats: { type: Object, default: () => ({}) },
    loading: { type: Boolean, default: false },
  },
  computed: {
    items() {
      const stats = this.stats || {}
      return [
        ['总 .strm', stats.total || 0],
        ['还没处理', stats.unprocessed || 0],
        ['已生成 .cas', stats.done || 0],
        ['已过滤', stats.filtered || 0],
        ['失败', stats.failed || 0],
        ['异常', stats.exception || 0],
        ['被跳过', stats.skipped || 0],
      ]
    },
  },
  template: `
    <div class="grid">
      <div v-for="([label, value], idx) in items" :key="idx" class="card">
        <div class="muted">{{ label }}</div>
        <div class="stat-value">{{ loading ? '加载中...' : value }}</div>
      </div>
    </div>
  `,
}
