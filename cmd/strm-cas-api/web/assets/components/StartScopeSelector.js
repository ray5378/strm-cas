export const StartScopeSelector = {
  props: {
    modelValue: { type: String, default: 'pending' },
    disabled: Boolean,
    loading: Boolean,
  },
  emits: ['update:modelValue', 'start'],
  template: `
    <div class="toolbar section">
      <strong>开始任务范围：</strong>
      <button @click="$emit('update:modelValue', 'pending')" :class="{ active: modelValue === 'pending' }" :disabled="disabled">只跑未处理</button>
      <button @click="$emit('update:modelValue', 'failed')" :class="{ active: modelValue === 'failed' }" :disabled="disabled">只跑失败</button>
      <button @click="$emit('update:modelValue', 'current_filter')" :class="{ active: modelValue === 'current_filter' }" :disabled="disabled">跑当前筛选结果</button>
      <button @click="$emit('start')" :disabled="disabled" :class="{ 'is-loading': loading }">{{ loading ? '启动中...' : '开始下载生成 CAS' }}</button>
    </div>
  `,
}
