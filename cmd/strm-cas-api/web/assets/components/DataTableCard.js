import { PagerControl } from './PagerControl.js'
import { EmptyState } from './EmptyState.js'

export const DataTableCard = {
  components: { PagerControl, EmptyState },
  props: {
    total: { type: Number, default: 0 },
    page: { type: Number, default: 1 },
    pageSize: { type: Number, default: 10 },
    loading: { type: Boolean, default: false },
    sectionClass: { type: String, default: '' },
    emptyTitle: { type: String, default: '暂无数据' },
    emptyMessage: { type: String, default: '' },
    errorMessage: { type: String, default: '' },
    emptyColspan: { type: Number, default: 1 },
    hidePagerWhenEmpty: { type: Boolean, default: false },
  },
  emits: ['prev', 'next', 'jump'],
  computed: {
    showPager() {
      if (!this.hidePagerWhenEmpty) return true
      return (this.total || 0) > 0
    },
  },
  template: `
    <div class="card" :class="[sectionClass, { 'panel-loading': loading }]">
      <slot name="header" />
      <table>
        <thead><slot name="thead" /></thead>
        <tbody>
          <EmptyState v-if="errorMessage" tone="error" title="加载失败" :message="errorMessage" :colspan="emptyColspan" />
          <template v-else>
            <slot name="rows" />
          </template>
        </tbody>
      </table>
      <PagerControl v-if="showPager" :total="total" :page="page" :page-size="pageSize" @prev="$emit('prev')" @next="$emit('next')" @jump="$emit('jump', $event)" />
    </div>
  `,
}
