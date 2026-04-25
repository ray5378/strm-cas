import { PagerControl } from './PagerControl.js'
import { stageText } from './ui.js'

export const DownloadedPanel = {
  components: { PagerControl },
  props: {
    downloaded: { type: Object, default: () => ({ total: 0, items: [] }) },
    page: { type: Number, default: 1 },
    loading: { type: Boolean, default: false },
  },
  emits: ['page-prev', 'page-next', 'page-jump'],
  methods: { stageText },
  template: `
    <div class="card section" :class="{ 'panel-loading': loading }">
      <strong>已下载任务</strong>
      <table>
        <thead><tr><th>阶段</th><th>文件</th><th>下载路径</th><th>更新时间</th></tr></thead>
        <tbody>
          <tr v-if="!(downloaded.items || []).length"><td colspan="4" class="muted">无数据</td></tr>
          <tr v-for="item in (downloaded.items || [])" :key="item.download_path + item.updated_at">
            <td>{{ stageText(item.stage) }}</td>
            <td>{{ item.file_name || '' }}</td>
            <td class="mono">{{ item.download_path || '' }}</td>
            <td>{{ item.updated_at || '' }}</td>
          </tr>
        </tbody>
      </table>
      <PagerControl :total="downloaded.total || 0" :page="page || 1" :page-size="10" @prev="$emit('page-prev')" @next="$emit('page-next')" @jump="$emit('page-jump', $event)" />
    </div>
  `,
}
