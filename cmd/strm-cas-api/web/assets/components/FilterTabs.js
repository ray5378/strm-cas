const FILTERS = [['','全部'], ['pending','未处理'], ['done','已完成'], ['failed','失败'], ['exception','异常'], ['skipped','已跳过']]

export const FilterTabs = {
  props: { modelValue: { type: String, default: '' } },
  emits: ['update:modelValue'],
  data() { return { filters: FILTERS } },
  template: `
    <div class="toolbar">
      <button
        v-for="([value, label], idx) in filters"
        :key="idx"
        @click="$emit('update:modelValue', value)"
        :class="{ active: modelValue === value }"
      >{{ label }}</button>
    </div>
  `,
}
