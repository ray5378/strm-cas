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
    emptyMessage: { type: String, default: '无数据' },
    emptyColspan: { type: Number, default: 1 },
  },
  emits: ['prev', 'next', 'jump'],
  template: `
    <div class="card" :class="[sectionClass, { 'panel-loading': loading }]">
      <slot name="header" />
      <table>
        <thead><slot name="thead" /></thead>
        <tbody>
          <EmptyState v-if="!$slots.rows" :message="emptyMessage" :colspan="emptyColspan" />
          <template v-else>
            <slot name="rows" />
          </template>
        </tbody>
      </table>
      <PagerControl :total="total" :page="page" :page-size="pageSize" @prev="$emit('prev')" @next="$emit('next')" @jump="$emit('jump', $event)" />
    </div>
  `,
}
