function field(label, value) {
  return { label, value: value ?? '' }
}

export const DetailPanel = {
  props: {
    detail: { type: Object, default: null },
    selectedPaths: { type: Array, default: () => [] },
    loading: { type: Object, default: () => ({}) },
  },
  emits: ['retry', 'toggle-selected', 'copy'],
  computed: {
    isSelected() {
      const path = this.detail?.strm_path
      return !!path && this.selectedPaths.includes(path)
    },
    basicInfo() {
      if (!this.detail) return []
      return [
        field('状态', this.detail.status),
        field('STRM 路径', this.detail.strm_path),
        field('URL', this.detail.url),
        field('相对目录', this.detail.relative_dir),
      ]
    },
    fileInfo() {
      if (!this.detail) return []
      return [
        field('CAS 路径', this.detail.cas_path),
        field('下载路径', this.detail.download_path),
        field('文件名', this.detail.file_name),
        field('文件大小', this.detail.size),
      ]
    },
    timeInfo() {
      if (!this.detail) return []
      return [
        field('最后处理时间', this.detail.last_processed_at),
        field('创建时间', this.detail.created_at),
        field('更新时间', this.detail.updated_at),
      ]
    },
    errorInfo() {
      if (!this.detail) return []
      return [
        field('最后消息', this.detail.last_message),
        field('错误信息', this.detail.error),
      ].filter(item => item.value !== '')
    },
    extraInfo() {
      if (!this.detail) return []
      const used = new Set(['status', 'strm_path', 'url', 'relative_dir', 'cas_path', 'download_path', 'file_name', 'size', 'last_processed_at', 'created_at', 'updated_at', 'last_message', 'error'])
      return Object.entries(this.detail)
        .filter(([key]) => !used.has(key))
        .map(([label, value]) => field(label, typeof value === 'string' ? value : JSON.stringify(value, null, 2)))
    },
  },
  template: `
    <div class="card">
      <strong>详情</strong>
      <div v-if="!detail" class="muted section">点击“详情”查看</div>
      <div v-else class="section detail-groups">
        <div class="toolbar">
          <button class="secondary" @click="$emit('toggle-selected', detail.strm_path)">{{ isSelected ? '取消选中' : '加入选中' }}</button>
          <button class="warning" v-if="detail.status === 'failed'" @click="$emit('retry', detail.strm_path)" :disabled="loading.retryOne === detail.strm_path" :class="{ 'is-loading': loading.retryOne === detail.strm_path }">{{ loading.retryOne === detail.strm_path ? '重试中...' : '重试当前项' }}</button>
          <button @click="$emit('copy', detail.strm_path)">复制 STRM 路径</button>
          <button v-if="detail.download_path" @click="$emit('copy', detail.download_path)">复制下载路径</button>
        </div>
        <section class="detail-group">
          <div class="detail-title">基本信息</div>
          <div v-for="item in basicInfo" :key="item.label" class="detail-row"><strong>{{ item.label }}：</strong><span class="mono">{{ item.value }}</span></div>
        </section>
        <section class="detail-group">
          <div class="detail-title">文件信息</div>
          <div v-for="item in fileInfo" :key="item.label" class="detail-row"><strong>{{ item.label }}：</strong><span class="mono">{{ item.value }}</span></div>
        </section>
        <section class="detail-group">
          <div class="detail-title">时间信息</div>
          <div v-for="item in timeInfo" :key="item.label" class="detail-row"><strong>{{ item.label }}：</strong><span class="mono">{{ item.value }}</span></div>
        </section>
        <section v-if="errorInfo.length" class="detail-group">
          <div class="detail-title">结果 / 错误</div>
          <div v-for="item in errorInfo" :key="item.label" class="detail-row"><strong>{{ item.label }}：</strong><span class="mono">{{ item.value }}</span></div>
        </section>
        <section v-if="extraInfo.length" class="detail-group">
          <div class="detail-title">其他字段</div>
          <div v-for="item in extraInfo" :key="item.label" class="detail-row"><strong>{{ item.label }}：</strong><span class="mono">{{ item.value }}</span></div>
        </section>
      </div>
    </div>
  `,
}
