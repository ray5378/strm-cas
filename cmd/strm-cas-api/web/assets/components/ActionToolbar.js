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
    settings: { type: Object, default: () => ({ concurrency: 2, total_rate_limit_mb: 0 }) },
  },
  emits: ['scan', 'reconcile-db', 'start', 'stop', 'stop-after-current', 'retry-failed', 'refresh', 'save-settings', 'set-mode', 'clear-step1', 'clear-step2', 'clear-cancel', 'toggle-auto-refresh', 'update-settings'],
  template: `
    <div class="section card">
      <div class="toolbar">
        <button @click="$emit('scan')" :disabled="running || loading.scan" :class="{ 'is-loading': loading.scan }">{{ loading.scan ? '扫描中...' : '扫描 /strm' }}</button>
        <button @click="$emit('reconcile-db')" :disabled="running || loading.reconcileDB" :class="{ 'is-loading': loading.reconcileDB }">{{ loading.reconcileDB ? '纠正中...' : '纠正数据库' }}</button>
        <span class="muted">扫描只更新数据库记录；纠正数据库会以当前 .strm 和实际存在的 .cas 为准修正状态</span>
      </div>
      <div class="toolbar section">
        <strong>运行设置：</strong>
        <label class="muted">并发数 <input type="number" min="1" :value="settings.concurrency" @input="$emit('update-settings', { key: 'concurrency', value: Number($event.target.value || 1) })" style="width:80px" /></label>
        <label class="muted">总限速(MB/s, 0=不限速) <input type="number" min="0" :value="settings.total_rate_limit_mb" @input="$emit('update-settings', { key: 'total_rate_limit_mb', value: Number($event.target.value || 0) })" style="width:100px" /></label>
        <button @click="$emit('save-settings')" :disabled="running || loading.saveSettings" :class="{ 'is-loading': loading.saveSettings }">{{ loading.saveSettings ? '保存中...' : '保存设置' }}</button>
      </div>
      <StartScopeSelector :model-value="startMode" :disabled="running || loading.start" :loading="loading.start" @update:modelValue="$emit('set-mode', $event)" @start="$emit('start')" />
      <div class="toolbar section">
        <button @click="$emit('retry-failed')" :disabled="running || loading.retryFailed" :class="{ 'is-loading': loading.retryFailed }">{{ loading.retryFailed ? '重试中...' : '批量重试失败任务' }}</button>
        <button v-if="running" @click="$emit('stop-after-current')" class="secondary" :disabled="loading.stopAfterCurrent || runtime.graceful_stopping" :class="{ 'is-loading': loading.stopAfterCurrent }">{{ runtime.graceful_stopping ? '已设置收尾停止' : (loading.stopAfterCurrent ? '设置中...' : '完成当前任务后停止') }}</button>
        <button v-if="running" @click="$emit('stop')" class="danger" :disabled="loading.stop" :class="{ 'is-loading': loading.stop }">{{ loading.stop ? '停止中...' : '停止任务' }}</button>
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
        <span class="muted">收尾停止: {{ runtime.graceful_stopping ? '已设置' : '未设置' }}</span>
        <span class="muted">并发: {{ settings.concurrency }}</span>
        <span class="muted">总限速: {{ settings.total_rate_limit_mb > 0 ? settings.total_rate_limit_mb + ' MB/s' : '不限速' }}</span>
        <span class="muted">已下载: {{ runtime.downloaded_count || 0 }}</span>
        <span class="muted">已完成: {{ runtime.completed_count || 0 }}</span>
      </div>
      <div v-if="confirmClear" class="section warn"><strong>二级确认：</strong>清理数据库会删除当前所有处理状态记录，但不会删除 /data/strm、/data/download 里的文件。</div>
    </div>
  `,
}
