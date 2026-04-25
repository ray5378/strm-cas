export const ConfirmDialog = {
  props: {
    visible: Boolean,
    title: { type: String, default: '请确认' },
    message: { type: String, default: '' },
    confirmText: { type: String, default: '确认' },
    cancelText: { type: String, default: '取消' },
    loading: Boolean,
  },
  emits: ['confirm', 'cancel'],
  template: `
    <div v-if="visible" class="dialog-mask">
      <div class="dialog-card card">
        <div class="dialog-title">{{ title }}</div>
        <div class="section">{{ message }}</div>
        <div class="toolbar section dialog-actions">
          <button class="secondary" @click="$emit('cancel')" :disabled="loading">{{ cancelText }}</button>
          <button class="danger" @click="$emit('confirm')" :disabled="loading" :class="{ 'is-loading': loading }">{{ loading ? '处理中...' : confirmText }}</button>
        </div>
      </div>
    </div>
  `,
}
