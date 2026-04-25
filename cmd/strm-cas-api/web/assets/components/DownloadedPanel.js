import { DataTableCard } from './DataTableCard.js'
import { EmptyState } from './EmptyState.js'
import { stageText } from './ui.js'

export const DownloadedPanel = {
  components: { DataTableCard, EmptyState },
  props: {
    downloaded: { type: Object, default: () => ({ total: 0, items: [] }) },
    page: { type: Number, default: 1 },
    loading: { type: Boolean, default: false },
  },
  emits: ['page-prev', 'page-next', 'page-jump'],
  methods: { stageText },
  template: `
    <DataTableCard
      :total="downloaded.total || 0"
      :page="page || 1"
      :page-size="10"
      :loading="loading"
      section-class="section"
      :empty-colspan="4"
      @prev="$emit('page-prev')"
      @next="$emit('page-next')"
      @jump="$emit('page-jump', $event)"
    >
      <template #header><strong>已下载任务</strong></template>
      <template #thead><tr><th>阶段</th><th>文件</th><th>下载路径</th><th>更新时间</th></tr></template>
      <template #rows>
        <EmptyState v-if="!(downloaded.items || []).length" :colspan="4" />
        <tr v-for="item in (downloaded.items || [])" :key="item.download_path + item.updated_at">
          <td>{{ stageText(item.stage) }}</td>
          <td>{{ item.file_name || '' }}</td>
          <td class="mono">{{ item.download_path || '' }}</td>
          <td>{{ item.updated_at || '' }}</td>
        </tr>
      </template>
    </DataTableCard>
  `,
}
