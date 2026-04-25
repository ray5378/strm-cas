export const DetailPanel = {
  props: { detail: { type: Object, default: null } },
  computed: {
    entries() { return this.detail ? Object.entries(this.detail) : [] },
  },
  template: `
    <div class="card">
      <strong>详情</strong>
      <div v-if="!detail" class="muted section">点击“详情”查看</div>
      <div v-else class="section mono">
        <div v-for="([key, value], idx) in entries" :key="idx"><strong>{{ key }}:</strong> {{ typeof value === 'string' ? value : JSON.stringify(value, null, 2) }}</div>
      </div>
    </div>
  `,
}
