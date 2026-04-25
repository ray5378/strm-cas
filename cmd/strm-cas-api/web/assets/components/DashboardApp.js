import { computed, onMounted, onBeforeUnmount, reactive } from '../vendor/vue.esm-browser.prod.js'
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
import { BatchActionsBar } from './BatchActionsBar.js'
import { ConfirmDialog } from './ConfirmDialog.js'

export const DashboardApp = {
  components: { StatsCards, ActionToolbar, CurrentTaskCard, RecordsPanel, DownloadedPanel, CompletedPanel, DetailPanel, ToastStack, BatchActionsBar, ConfirmDialog },
  setup() {
    const store = useDashboardStore()
    const toast = useToast()
    const runtime = computed(() => store.state.overview?.runtime || {})
    const stats = computed(() => store.state.overview?.stats || {})
    const autoRefreshLabel = computed(() => runtime.value?.running ? '运行中 3s 自动刷新' : '空闲 15s 自动刷新')
    const confirmState = reactive({ visible: false, title: '', message: '', confirmText: '确认', action: null })
    let timer = null

    const toastResult = (res, fallback) => {
      if (res && typeof res.started === 'number') {
        const requested = typeof res.requested === 'number' ? res.requested : res.started
        const matched = typeof res.matched === 'number' ? res.matched : res.started
        const skipped = typeof res.skipped === 'number' ? res.skipped : Math.max(0, requested - res.started)
        toast.success(`${fallback}：请求 ${requested} 项，匹配 ${matched} 项，加入 ${res.started} 项，跳过 ${skipped} 项`)
        return
      }
      if (res && typeof res.stopped === 'boolean') {
        toast.success(res.stopped ? '当前任务已发出停止请求' : '当前没有运行中的任务')
        return
      }
      if (res && typeof res.total_strm === 'number') {
        toast.success(`数据库纠正完成：STRM ${res.total_strm}，发现 CAS ${res.cas_found}，标记完成 ${res.marked_done}，改回待处理 ${res.marked_pending}，异常 ${res.marked_exception}，删除过期记录 ${res.removed_stale}`)
        return
      }
      toast.success(fallback)
    }

    const runAction = async (fn, successMessage) => {
      try {
        const res = await fn()
        if (typeof successMessage === 'function') successMessage(res)
        else if (successMessage) toast.success(successMessage)
        return res
      } catch (e) {
        toast.error(e.message || String(e))
      }
    }

    const openConfirm = (title, message, action, confirmText = '确认') => {
      confirmState.visible = true
      confirmState.title = title
      confirmState.message = message
      confirmState.action = action
      confirmState.confirmText = confirmText
    }
    const closeConfirm = () => {
      confirmState.visible = false
      confirmState.title = ''
      confirmState.message = ''
      confirmState.action = null
      confirmState.confirmText = '确认'
    }
    const confirmAndRun = async () => {
      if (!confirmState.action) return
      const action = confirmState.action
      closeConfirm()
      await action()
    }

    const copyText = async (text) => {
      if (!text) return
      try {
        await navigator.clipboard.writeText(text)
        toast.success('已复制到剪贴板')
      } catch {
        toast.error('复制失败，请手动复制')
      }
    }

    const updateSettingsField = ({ key, value }) => {
      store.state.settings = { ...store.state.settings, [key]: value }
    }

    const scheduleRefresh = () => {
      if (timer) clearTimeout(timer)
      if (!store.state.autoRefreshEnabled) return
      const delay = runtime.value?.running ? 3000 : 15000
      timer = setTimeout(async () => {
        try {
          await store.refreshOverview()
          await store.refreshDownloaded()
          await store.refreshCompleted()
        } finally {
          scheduleRefresh()
        }
      }, delay)
    }

    const toggleAutoRefresh = () => {
      store.state.autoRefreshEnabled = !store.state.autoRefreshEnabled
      toast.info(store.state.autoRefreshEnabled ? '自动刷新已开启' : '自动刷新已关闭')
      scheduleRefresh()
    }

    const confirmBatchStartSelected = () => openConfirm('开始选中项', `即将开始 ${store.state.selectedPaths.length} 个选中任务，是否继续？`, () => runAction(() => store.startSelected(), res => toastResult(res, '选中任务已加入队列')), '开始任务')
    const confirmBatchRetrySelected = () => openConfirm('重试选中失败项', `即将重试 ${store.state.selectedPaths.length} 个选中项中的失败任务，是否继续？`, () => runAction(() => store.retrySelected(), res => toastResult(res, '选中失败任务已重新加入队列')), '开始重试')
    const confirmBatchStartFilter = () => openConfirm('按当前筛选开始任务', '将按当前筛选条件批量启动任务，是否继续？', () => runAction(() => store.startCurrentFilter(), res => toastResult(res, '当前筛选任务已加入队列')), '开始任务')
    const confirmBatchRetryFilter = () => openConfirm('按当前筛选重试失败', '将按当前筛选条件批量重试失败任务，是否继续？', () => runAction(() => store.retryByFilter(), res => toastResult(res, '当前筛选下的失败任务已重新加入队列')), '开始重试')
    const confirmStopTasks = () => openConfirm('停止当前任务', '将停止当前正在运行的批次任务，是否继续？', () => runAction(() => store.stopTasks(), res => toastResult(res, '停止请求已发出')), '停止任务')

    onMounted(async () => {
      store.state.loading.initial = true
      try {
        await runAction(() => store.refreshAll())
      } finally {
        store.state.loading.initial = false
      }
      scheduleRefresh()
    })

    onBeforeUnmount(() => {
      if (timer) clearTimeout(timer)
    })

    return { store, runtime, stats, toast, runAction, autoRefreshLabel, toggleAutoRefresh, confirmState, openConfirm, closeConfirm, confirmAndRun, copyText, updateSettingsField, confirmBatchStartSelected, confirmBatchRetrySelected, confirmBatchStartFilter, confirmBatchRetryFilter, confirmStopTasks }
  },
  template: `
    <div class="layout">
      <ToastStack :items="toast.items" />
      <ConfirmDialog
        :visible="confirmState.visible"
        :title="confirmState.title"
        :message="confirmState.message"
        :confirm-text="confirmState.confirmText"
        :loading="store.state.loading.start || store.state.loading.retryFailed || store.state.loading.stop"
        @confirm="confirmAndRun"
        @cancel="closeConfirm"
      />
      <div class="title">strm-cas 控制台</div>
      <div v-if="store.state.error" class="card" style="background:#fee2e2;color:#991b1b">{{ store.state.error }}</div>
      <div v-if="store.state.loading.initial" class="card">页面初始化加载中...</div>
      <div v-if="store.state.errors.overview" class="card" style="background:#fff7ed;color:#9a3412">概览刷新失败：{{ store.state.errors.overview }}</div>
      <StatsCards :stats="stats" />
      <ActionToolbar
        :running="!!runtime.running"
        :runtime="runtime"
        :start-mode="store.state.startMode"
        :confirm-clear="store.state.confirmClear"
        :loading="store.state.loading"
        :auto-refresh-enabled="store.state.autoRefreshEnabled"
        :auto-refresh-label="autoRefreshLabel"
        :settings="store.state.settings"
        @scan="runAction(() => store.scan(), '扫描完成')"
        @reconcile-db="runAction(() => store.reconcileDB(), res => toastResult(res, '数据库已纠正'))"
        @start="runAction(() => store.start(), res => toastResult(res, '任务已启动'))"
        @stop="confirmStopTasks"
        @retry-failed="runAction(() => store.retryFailed(), res => toastResult(res, '失败任务已重新加入队列'))"
        @refresh="runAction(() => store.refreshAll(), '已刷新')"
        @save-settings="runAction(() => store.saveSettings(), '设置已保存，将作用于后续启动的任务')"
        @update-settings="updateSettingsField"
        @toggle-auto-refresh="toggleAutoRefresh"
        @set-mode="store.state.startMode = $event"
        @clear-step1="store.state.confirmClear = true"
        @clear-step2="runAction(() => store.clearDB(), '数据库已清理')"
        @clear-cancel="store.state.confirmClear = false"
      />
      <CurrentTaskCard class="section" :current="runtime.current || null" />
      <div class="main-grid section">
        <div>
          <BatchActionsBar
            :filters="store.state.filters"
            :loading="store.state.loading"
            :selected-count="store.state.selectedPaths.length"
            @start-current-filter="confirmBatchStartFilter"
            @retry-current-filter="confirmBatchRetryFilter"
            @start-selected="confirmBatchStartSelected"
            @retry-selected="confirmBatchRetrySelected"
            @clear-selected="store.clearSelected()"
          />
          <RecordsPanel
            :records="store.state.records"
            :filters="store.state.filters"
            :loading="store.state.loading"
            :error-message="store.state.errors.records"
            :selected-paths="store.state.selectedPaths"
            @toggle-selected="store.toggleSelected($event)"
            @toggle-select-all="store.toggleSelectAllCurrentPage($event)"
            @set-status="(v) => { store.state.filters.status = v; store.state.filters.page = 1; runAction(() => store.refreshRecords()) }"
            @apply-search="(v) => { store.state.filters.search = v; store.state.filters.page = 1; runAction(() => store.refreshRecords(), '筛选已更新') }"
            @detail="(path) => runAction(() => store.loadDetail(path))"
            @retry="(path) => runAction(() => store.retryOne(path), res => toastResult(res, '任务已重新加入队列'))"
            @page-prev="() => { if (store.state.filters.page > 1) { store.state.filters.page--; runAction(() => store.refreshRecords()) } }"
            @page-next="() => { store.state.filters.page++; runAction(() => store.refreshRecords()) }"
            @page-jump="(v) => { store.state.filters.page = v; runAction(() => store.refreshRecords()) }"
          />
          <DownloadedPanel
            :downloaded="store.state.downloaded"
            :page="store.state.downloadedPage"
            :loading="store.state.loading.downloaded"
            :error-message="store.state.errors.downloaded"
            @detail="(path) => runAction(() => store.loadDetail(path))"
            @page-prev="() => { if (store.state.downloadedPage > 1) { store.state.downloadedPage--; runAction(() => store.refreshDownloaded()) } }"
            @page-next="() => { store.state.downloadedPage++; runAction(() => store.refreshDownloaded()) }"
            @page-jump="(v) => { store.state.downloadedPage = v; runAction(() => store.refreshDownloaded()) }"
          />
          <CompletedPanel
            :completed="store.state.completed"
            :status="store.state.completedStatus"
            :page="store.state.completedPage"
            :loading="store.state.loading"
            :error-message="store.state.errors.completed"
            @set-status="(v) => { store.state.completedStatus = v; store.state.completedPage = 1; runAction(() => store.refreshCompleted()) }"
            @detail="(path) => runAction(() => store.loadDetail(path))"
            @retry="(path) => runAction(() => store.retryOne(path), res => toastResult(res, '任务已重新加入队列'))"
            @page-prev="() => { if (store.state.completedPage > 1) { store.state.completedPage--; runAction(() => store.refreshCompleted()) } }"
            @page-next="() => { store.state.completedPage++; runAction(() => store.refreshCompleted()) }"
            @page-jump="(v) => { store.state.completedPage = v; runAction(() => store.refreshCompleted()) }"
          />
        </div>
        <div>
          <DetailPanel
            :detail="store.state.detail"
            :selected-paths="store.state.selectedPaths"
            :loading="store.state.loading"
            @toggle-selected="store.toggleSelected($event)"
            @retry="(path) => runAction(() => store.retryOne(path), res => toastResult(res, '任务已重新加入队列'))"
            @copy="copyText"
          />
        </div>
      </div>
    </div>
  `,
}
gleSelected($event)"
            @retry="(path) => runAction(() => store.retryOne(path), res => toastResult(res, '任务已重新加入队列'))"
            @copy="copyText"
          />
        </div>
      </div>
    </div>
  `,
}
