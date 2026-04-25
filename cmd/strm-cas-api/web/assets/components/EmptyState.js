export const EmptyState = {
  props: {
    title: { type: String, default: '暂无数据' },
    message: { type: String, default: '' },
    colspan: { type: Number, default: 1 },
    tone: { type: String, default: 'empty' },
  },
  template: `
    <tr>
      <td :colspan="colspan">
        <div class="empty-state" :class="tone">
          <div class="empty-title">{{ title }}</div>
          <div v-if="message" class="muted">{{ message }}</div>
        </div>
      </td>
    </tr>
  `,
}
