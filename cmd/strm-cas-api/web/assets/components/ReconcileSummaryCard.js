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
    details() {
      return Array.isArray(this.summary?.details) ? this.summary.details : []
    },
  },
  methods: {
    actionLabel(action) {
      switch (action) {
        case 'mark_done': return '改成 done'
        case 'mark_pending': return '改回 pending'
        case 'mark_exception': return '改成 exception'
        case 'delete_stale': return '删除陈旧记录'
        default: return action || '-'
      }
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
      <div v-if="details.length" class="section">
        <strong>本次变更明细</strong>
        <div class="section" style="display:flex;flex-direction:column;gap:10px;max-height:520px;overflow:auto">
          <div v-for="(item, idx) in details" :key="idx" class="card">
            <div><strong>{{ actionLabel(item.action) }}</strong></div>
            <div class="detail-row"><strong>STRM：</strong><span class="mono">{{ item.strm_path || '-' }}</span></div>
            <div class="detail-row"><strong>相对目录：</strong><span class="mono">{{ item.relative_dir || '-' }}</span></div>
            <div class="detail-row"><strong>状态：</strong><span class="mono">{{ item.old_status || '-' }} → {{ item.new_status || '-' }}</span></div>
            <div v-if="item.match_mode" class="detail-row"><strong>匹配方式：</strong><span class="mono">{{ item.match_mode }}</span></div>
            <div v-if="item.cas_path" class="detail-row"><strong>CAS：</strong><span class="mono">{{ item.cas_path }}</span></div>
            <div v-if="item.message" class="detail-row"><strong>说明：</strong><span class="mono">{{ item.message }}</span></div>
          </div>
        </div>
      </div>
    </div>
  `,
}
