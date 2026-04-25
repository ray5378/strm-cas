import { StartScopeSelector } from './StartScopeSelector.js'

export const ActionToolbar = {
  components: { StartScopeSelector },
  props: {
    running: Boolean,
    runtime: { type: Object, default: () => ({}) },
    startMode: { type: String, default: 'pending' },
    confirmClear: Boolean,
  },
  emits: ['scan', 'start', 'retry-failed', 'refresh', 'set-mode', 'clear-step1', 'clear-step2', 'clear-cancel'],
  template: `
    <div class="section card">
      <div class="toolbar">
        <button @click="$emit('scan')" :disabled="running">扫描 /strm</button>
        <span class="muted">扫描只更新数据库记录，不执行下载</span>
      </div>
      <StartScopeSelector :model-value="startMode" @update:modelValue="$emit('set-mode', $event)" @start="$emit('start')" />
      <div class="toolbar section">
        <button @click="$emit('retry-failed')" :disabled="running">批量重试失败任务</button>
        <button @click="$emit('refresh')">刷新</button>
        <template v-if="!confirmClear">
          <button @click="$emit('clear-step1')" :disabled="running" class="danger">清理数据库</button>
        </template>
        <template v-else>
          <button @click="$emit('clear-step2')" :disabled="running" class="danger-dark">确认清理数据库</button>
          <button @click="$emit('clear-cancel')" class="secondary">取消</button>
        </template>
        <span class="muted">运行中: {{ running ? '是' : '否' }}</span>
        <span class="muted">已下载: {{ runtime.downloaded_count || 0 }}</span>
        <span class="muted">已完成: {{ runtime.completed_count || 0 }}</span>
      </div>
      <div v-if="confirmClear" class="section warn"><strong>二级确认：</strong>清理数据库会删除当前所有处理状态记录，但不会删除 /data/strm、/data/download 里的文件。</div>
    </div>
  `,
}
