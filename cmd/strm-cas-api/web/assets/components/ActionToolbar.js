import { StartScopeSelector } from './StartScopeSelector.js'

export const ActionToolbar = {
  components: { StartScopeSelector },
  props: {
    running: Boolean,
    runtime: { type: Object, default: () => ({}) },
    startMode: { type: String, default: 'pending' },
    confirmClear: Boolean,
    loading: { type: Object, default: () => ({}) },
    autoRefreshEnabled: { type: Boolean, default: true },
    autoRefreshLabel: { type: String, default: '空闲 15s / 运行中 3s' },
  },
  emits: ['scan', 'start', 'retry-failed', 'refresh', 'set-mode', 'clear-step1', 'clear-step2', 'clear-cancel', 'toggle-auto-refresh'],
  template: `
    <div class="section card">
      <div class="toolbar">
        <button @click="$emit('scan')" :disabled="running || loading.scan" :class="{ 'is-loading': loading.scan }">{{ loading.scan ? '扫描中...' : '扫描 /strm' }}</button>
        <span class="muted">扫描只更新数据库记录，不执行下载</span>
      </div>
      <StartScopeSelector :model-value="startMode" :disabled="running || loading.start" :loading="loading.start" @update:modelValue="$emit('set-mode', $event)" @start="$emit('start')" />
      <div class="toolbar section">
        <button @click="$emit('retry-failed')" :disabled="running || loading.retryFailed" :class="{ 'is-loading': loading.retryFailed }">{{ loading.retryFailed ? '重试中...' : '批量重试失败任务' }}</button>
        <button @click="$emit('refresh')" :disabled="loading.refreshAll" :class="{ 'is-loading': loading.refreshAll }">{{ loading.refreshAll ? '刷新中...' : '刷新' }}</button>
        <button @click="$emit('toggle-auto-refresh')" class="secondary">自动刷新：{{ autoRefreshEnabled ? '开' : '关' }}</button>
        <span class="muted">{{ autoRefreshLabel }}</span>
        <template v-if="!confirmClear">
          <button @click="$emit('clear-step1')" :disabled="running || loading.clearDB" class="danger">清理数据库</button>
        </template>
        <template v-else>
          <button @click="$emit('clear-step2')" :disabled="running || loading.clearDB" class="danger-dark" :class="{ 'is-loading': loading.clearDB }">{{ loading.clearDB ? '清理中...' : '确认清理数据库' }}</button>
          <button @click="$emit('clear-cancel')" class="secondary" :disabled="loading.clearDB">取消</button>
        </template>
        <span class="muted">运行中: {{ running ? '是' : '否' }}</span>
        <span class="muted">已下载: {{ runtime.downloaded_count || 0 }}</span>
        <span class="muted">已完成: {{ runtime.completed_count || 0 }}</span>
      </div>
      <div v-if="confirmClear" class="section warn"><strong>二级确认：</strong>清理数据库会删除当前所有处理状态记录，但不会删除 /data/strm、/data/download 里的文件。</div>
    </div>
  `,
}
