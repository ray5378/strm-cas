export const ReconcileSummaryCard = {
  props: {
    summary: { type: Object, default: null },
  },
  computed: {
    items() {
      if (!this.summary) return []
      return [
        ['本次 STRM', this.summary.total_strm || 0],
        ['本次 CAS', this.summary.total_cas || 0],
        ['纠正为 done', this.summary.done || 0],
        ['纠正为 pending', this.summary.pending || 0],
        ['纠正为 exception', this.summary.exception || 0],
        ['更新记录', this.summary.updated || 0],
        ['删除陈旧记录', this.summary.deleted_stale || 0],
        ['命中已有 CASPath', this.summary.matched_existing || 0],
        ['推断匹配成功', this.summary.matched_inferred || 0],
      ]
    },
  },
  template: `
    <div v-if="summary" class="card section">
      <strong>最近一次纠正结果</strong>
      <div class="grid section">
        <div v-for="([label, value], idx) in items" :key="idx" class="card">
          <div class="muted">{{ label }}</div>
          <div class="stat-value">{{ value }}</div>
        </div>
      </div>
      <div class="muted">说明：以当前 .strm 发现结果和 download 目录里真实存在的 .cas 文件为准，对数据库状态做保守纠正。</div>
    </div>
  `,
}
