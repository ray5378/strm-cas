import { PagerControl } from './PagerControl.js'
import { statusText } from './ui.js'

const FILTERS = [['','全部'], ['pending','未处理'], ['done','已完成'], ['failed','失败'], ['exception','异常'], ['skipped','已跳过']]

export const RecordsPanel = {
  components: { PagerControl },
  props: {
    records: { type: Object, default: () => ({ total: 0, items: [] }) },
    filters: { type: Object, required: true },
  },
  emits: ['set-status', 'apply-search', 'detail', 'retry', 'page-prev', 'page-next', 'page-jump'],
  data() {
    return { searchValue: this.filters.search || '' }
  },
  watch: {
    'filters.search'(v) { this.searchValue = v || '' },
  },
  methods: { statusText },
  template: `
    <div class="card">
      <div class="toolbar">
        <strong>数据库记录</strong>
        <button v-for="([value, label], idx) in ${JSON.stringify(FILTERS)}" :key="idx" @click="$emit('set-status', value)" :class="{ active: filters.status === value }">{{ label }}</button>
        <input v-model="searchValue" placeholder="搜索路径 / URL / 错误" />
        <button @click="$emit('apply-search', searchValue)">筛选</button>
      </div>
      <table>
        <thead><tr><th>状态</th><th>strm</th><th>cas</th><th>最后结果</th><th></th></tr></thead>
        <tbody>
          <tr v-if="!(records.items || []).length"><td colspan="5" class="muted">无数据</td></tr>
          <tr v-for="item in (records.items || [])" :key="item.strm_path">
            <td><span class="badge" :class="item.status || 'pending'">{{ statusText(item.status) }}</span></td>
            <td class="mono">{{ item.strm_path }}</td>
            <td class="mono">{{ item.cas_path || '' }}</td>
            <td>{{ item.last_message || '' }}</td>
            <td>
              <button @click="$emit('detail', item.strm_path)">详情</button>
              <button v-if="item.status === 'failed'" @click="$emit('retry', item.strm_path)" class="warning">重试</button>
            </td>
          </tr>
        </tbody>
      </table>
      <PagerControl :total="records.total || 0" :page="filters.page || 1" :page-size="filters.page_size || 10" @prev="$emit('page-prev')" @next="$emit('page-next')" @jump="$emit('page-jump', $event)" />
    </div>
  `,
}
