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
      store.state.loading.initial = true
      try {
        await runAction(() => store.refreshAll())
      } finally {
        store.state.loading.initial = false
      }
      setInterval(refreshLoop, 3000)
    })

    return { store, runtime, stats, toast, runAction }
  },
  template: `
    <div class="layout">
      <ToastStack :items="toast.items" />
      <div class="title">strm-cas 控制台</div>
      <div v-if="store.state.error" class="card" style="background:#fee2e2;color:#991b1b">{{ store.state.error }}</div>
      <div v-if="store.state.loading.initial" class="card">页面初始化加载中...</div>
      <StatsCards :stats="stats" />
      <ActionToolbar
        :running="!!runtime.running"
        :runtime="runtime"
        :start-mode="store.state.startMode"
        :confirm-clear="store.state.confirmClear"
        :loading="store.state.loading"
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
            :loading="store.state.loading"
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
            :loading="store.state.loading.downloaded"
            @page-prev="() => { if (store.state.downloadedPage > 1) { store.state.downloadedPage--; runAction(() => store.refreshDownloaded()) } }"
            @page-next="() => { store.state.downloadedPage++; runAction(() => store.refreshDownloaded()) }"
            @page-jump="(v) => { store.state.downloadedPage = v; runAction(() => store.refreshDownloaded()) }"
          />
          <CompletedPanel
            :completed="store.state.completed"
            :status="store.state.completedStatus"
            :page="store.state.completedPage"
            :loading="store.state.loading"
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
