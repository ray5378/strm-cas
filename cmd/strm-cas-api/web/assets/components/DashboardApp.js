import { computed, onMounted } from '../vendor/vue.esm-browser.prod.js'
import { useDashboardStore } from '../composables/useDashboardStore.js'
import { useToast } from '../composables/useToast.js'
import { StatsCards } from './StatsCards.js'
import { ActionToolbar } from './ActionToolbar.js'
import { CurrentTaskCard } from './CurrentTaskCard.js'
import { RecordsPanel } from './RecordsPanel.js'
import { DownloadedPanel } from './DownloadedPanel.js'
import { CompletedPanel } from './CompletedPanel.js'
import { DetailPanel } from './DetailPanel.js'
import { ToastStack } from './ToastStack.js'

export const DashboardApp = {
  components: { StatsCards, ActionToolbar, CurrentTaskCard, RecordsPanel, DownloadedPanel, CompletedPanel, DetailPanel, ToastStack },
  setup() {
    const store = useDashboardStore()
    const toast = useToast()
    const runtime = computed(() => store.state.overview?.runtime || {})
    const stats = computed(() => store.state.overview?.stats || {})

    const runAction = async (fn, successMessage) => {
      try {
        const res = await fn()
        if (successMessage) toast.success(successMessage)
        return res
      } catch (e) {
        toast.error(e.message || String(e))
      }
    }

    const refreshLoop = async () => {
      try {
        await store.refreshOverview()
        await store.refreshDownloaded()
        await store.refreshCompleted()
      } catch {}
    }

    onMounted(async () => {
      await runAction(() => store.refreshAll())
      setInterval(refreshLoop, 3000)
    })

    return { store, runtime, stats, toast, runAction }
  },
  template: `
    <div class="layout">
      <style>
        .layout { max-width: 1400px; margin: 0 auto; padding: 20px; }
        .grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: 12px; }
        .card { background: #fff; border-radius: 12px; padding: 16px; box-shadow: 0 1px 3px rgba(0,0,0,.08); }
        .toolbar, .row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
        .records-toolbar { align-items: flex-start; }
        .grow { flex: 1; min-width: 260px; }
        .grow-input { flex: 1; min-width: 220px; }
        button { padding: 8px 12px; border: 0; border-radius: 8px; background: #2563eb; color: white; cursor: pointer; }
        button.active { background: #1d4ed8; }
        button.secondary { background: #64748b; }
        button.warning { background: #ea580c; }
        button.danger { background: #dc2626; }
        button.danger-dark { background: #b91c1c; }
        button:disabled { background: #94a3b8; cursor: not-allowed; }
        input, select { padding: 8px 10px; border: 1px solid #cbd5e1; border-radius: 8px; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 10px; border-bottom: 1px solid #e5e7eb; text-align: left; vertical-align: top; }
        .section { margin-top: 16px; }
        .title { font-size: 24px; font-weight: 700; margin-bottom: 12px; }
        .muted { color: #64748b; font-size: 12px; }
        .mono { font-family: monospace; word-break: break-all; white-space: pre-wrap; }
        .badge { display:inline-block; padding:2px 8px; border-radius:999px; font-size:12px; }
        .done { background:#dcfce7; color:#166534; }
        .failed { background:#fee2e2; color:#991b1b; }
        .exception { background:#fde68a; color:#92400e; }
        .pending { background:#fef3c7; color:#92400e; }
        .skipped { background:#e0e7ff; color:#3730a3; }
        .main-grid { display:grid; grid-template-columns: 1.2fr .8fr; gap:16px; }
        .stat-value { font-size:30px; font-weight:700; }
        .warn { color:#991b1b; }
        .pager-wrap { display:flex; justify-content:space-between; gap:12px; align-items:center; margin-top:12px; flex-wrap:wrap; }
        .toast-stack { position: fixed; top: 16px; right: 16px; display: flex; flex-direction: column; gap: 10px; z-index: 9999; }
        .toast { min-width: 220px; max-width: 420px; color: #fff; padding: 12px 14px; border-radius: 10px; box-shadow: 0 8px 24px rgba(0,0,0,.16); }
        .toast.success { background: #166534; }
        .toast.error { background: #991b1b; }
        .toast.info { background: #1d4ed8; }
        @media (max-width: 1100px){ .grid, .main-grid { grid-template-columns: 1fr; } }
      </style>
      <ToastStack :items="toast.items" />
      <div class="title">strm-cas 控制台</div>
      <div v-if="store.state.error" class="card" style="background:#fee2e2;color:#991b1b">{{ store.state.error }}</div>
      <StatsCards :stats="stats" />
      <ActionToolbar
        :running="!!runtime.running"
        :runtime="runtime"
        :start-mode="store.state.startMode"
        :confirm-clear="store.state.confirmClear"
        @scan="runAction(() => store.scan(), '扫描完成')"
        @start="runAction(() => store.start(), '任务已启动')"
        @retry-failed="runAction(() => store.retryFailed(), '失败任务已重新加入队列')"
        @refresh="runAction(() => store.refreshAll(), '已刷新')"
        @set-mode="store.state.startMode = $event"
        @clear-step1="store.state.confirmClear = true"
        @clear-step2="runAction(() => store.clearDB(), '数据库已清理')"
        @clear-cancel="store.state.confirmClear = false"
      />
      <CurrentTaskCard class="section" :current="runtime.current || null" />
      <div class="main-grid section">
        <div>
          <RecordsPanel
            :records="store.state.records"
            :filters="store.state.filters"
            @set-status="(v) => { store.state.filters.status = v; store.state.filters.page = 1; runAction(() => store.refreshRecords()) }"
            @apply-search="(v) => { store.state.filters.search = v; store.state.filters.page = 1; runAction(() => store.refreshRecords(), '筛选已更新') }"
            @detail="(path) => runAction(() => store.loadDetail(path))"
            @retry="(path) => runAction(() => store.retryOne(path), '任务已重新加入队列')"
            @page-prev="() => { if (store.state.filters.page > 1) { store.state.filters.page--; runAction(() => store.refreshRecords()) } }"
            @page-next="() => { store.state.filters.page++; runAction(() => store.refreshRecords()) }"
            @page-jump="(v) => { store.state.filters.page = v; runAction(() => store.refreshRecords()) }"
          />
          <DownloadedPanel
            :downloaded="store.state.downloaded"
            :page="store.state.downloadedPage"
            @page-prev="() => { if (store.state.downloadedPage > 1) { store.state.downloadedPage--; runAction(() => store.refreshDownloaded()) } }"
            @page-next="() => { store.state.downloadedPage++; runAction(() => store.refreshDownloaded()) }"
            @page-jump="(v) => { store.state.downloadedPage = v; runAction(() => store.refreshDownloaded()) }"
          />
          <CompletedPanel
            :completed="store.state.completed"
            :status="store.state.completedStatus"
            :page="store.state.completedPage"
            @set-status="(v) => { store.state.completedStatus = v; store.state.completedPage = 1; runAction(() => store.refreshCompleted()) }"
            @retry="(path) => runAction(() => store.retryOne(path), '任务已重新加入队列')"
            @page-prev="() => { if (store.state.completedPage > 1) { store.state.completedPage--; runAction(() => store.refreshCompleted()) } }"
            @page-next="() => { store.state.completedPage++; runAction(() => store.refreshCompleted()) }"
            @page-jump="(v) => { store.state.completedPage = v; runAction(() => store.refreshCompleted()) }"
          />
        </div>
        <div>
          <DetailPanel :detail="store.state.detail" />
        </div>
      </div>
    </div>
  `,
}
