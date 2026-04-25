export const StartScopeSelector = {
  props: { modelValue: { type: String, default: 'pending' } },
  emits: ['update:modelValue', 'start'],
  template: `
    <div class="toolbar section">
      <strong>开始任务范围：</strong>
      <button @click="$emit('update:modelValue', 'pending')" :class="{ active: modelValue === 'pending' }">只跑未处理</button>
      <button @click="$emit('update:modelValue', 'failed')" :class="{ active: modelValue === 'failed' }">只跑失败</button>
      <button @click="$emit('update:modelValue', 'current_filter')" :class="{ active: modelValue === 'current_filter' }">跑当前筛选结果</button>
      <button @click="$emit('start')">开始下载生成 CAS</button>
    </div>
  `,
}
