export const EmptyState = {
  props: {
    message: { type: String, default: '无数据' },
    colspan: { type: Number, default: 1 },
  },
  template: `<tr><td :colspan="colspan" class="muted">{{ message }}</td></tr>`,
}
