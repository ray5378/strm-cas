import { reactive } from '../vendor/vue.esm-browser.prod.js'
import { dashboardService } from '../services/dashboardService.js'

export function useDashboardStore() {
  const state = reactive({
    overview: null,
    settings: { concurrency: 2, total_rate_limit_mb: 0 },
    records: { total: 0, items: [] },
    downloaded: { total: 0, items: [] },
    completed: { total: 0, items: [] },
    detail: null,
    reconcileSummary: null,
    selectedPaths: [],
    filters: { status: '', search: '', page: 1, page_size: 10 },
    downloadedPage: 1,
    completedPage: 1,
    completedStatus: '',
    startMode: 'pending',
    error: '',
    errors: { records: '', downloaded: '', completed: '', detail: '', overview: '' },
    confirmClear: false,
    autoRefreshEnabled: true,
    loading: {
      initial: false,
      refreshAll: false,
      scan: false,
      start: false,
      stop: false,
      saveSettings: false,
      retryFailed: false,
      reconcileDB: false,
      clearDB: false,
      records: false,
      downloaded: false,
      completed: false,
      detail: false,
      retryOne: '',
    },
  })

  async function wrap(action, loadingKey) {
    try {
      state.error = ''
      if (loadingKey) state.loading[loadingKey] = true
      return await action()
    } catch (e) {
      state.error = e.message || String(e)
      throw e
    } finally {
      if (loadingKey) state.loading[loadingKey] = false
    }
  }

  function toggleSelected(path) {
    const idx = state.selectedPaths.indexOf(path)
    if (idx >= 0) state.selectedPaths.splice(idx, 1)
    else state.selectedPaths.push(path)
  }
  function clearSelected() { state.selectedPaths.splice(0, state.selectedPaths.length) }
  function toggleSelectAllCurrentPage(paths) {
    const valid = paths.filter(Boolean)
    const allSelected = valid.length > 0 && valid.every(path => state.selectedPaths.includes(path))
    if (allSelected) {
      for (const path of valid) {
        const idx = state.selectedPaths.indexOf(path)
        if (idx >= 0) state.selectedPaths.splice(idx, 1)
      }
      return
    }
    for (const path of valid) if (!state.selectedPaths.includes(path)) state.selectedPaths.push(path)
  }

  async function refreshOverview() {
    try {
      state.overview = await dashboardService.overview()
      state.errors.overview = ''
      if (state.overview?.settings) {
        state.settings = {
          concurrency: state.overview.settings.concurrency || 1,
          total_rate_limit_mb: state.overview.settings.total_rate_limit_mb || 0,
        }
      }
    } catch (e) {
      state.errors.overview = e.message || String(e)
      throw e
    }
  }
  async function refreshSettings() {
    return wrap(async () => {
      const s = await dashboardService.settings()
      state.settings = { concurrency: s.concurrency || 1, total_rate_limit_mb: s.total_rate_limit_mb || 0 }
    }, 'saveSettings')
  }
  async function saveSettings() {
    return wrap(async () => {
      const s = await dashboardService.saveSettings(state.settings)
      state.settings = { concurrency: s.concurrency || 1, total_rate_limit_mb: s.total_rate_limit_mb || 0 }
      return s
    }, 'saveSettings')
  }
  async function refreshRecords() {
    return wrap(async () => {
      state.records = await dashboardService.records(state.filters)
      state.errors.records = ''
    }, 'records')
  }
  async function refreshDownloaded() {
    return wrap(async () => {
      state.downloaded = await dashboardService.runtimeDownloaded({ page: state.downloadedPage, page_size: 10 })
      state.errors.downloaded = ''
    }, 'downloaded')
  }
  async function refreshCompleted() {
    return wrap(async () => {
      state.completed = await dashboardService.runtimeCompleted({ page: state.completedPage, page_size: 10, status: state.completedStatus })
      state.errors.completed = ''
    }, 'completed')
  }
  async function refreshAll() {
    return wrap(async () => {
      await Promise.all([refreshOverview(), refreshRecords(), refreshDownloaded(), refreshCompleted()])
    }, 'refreshAll')
  }
  async function loadDetail(path) {
    return wrap(async () => {
      state.detail = await dashboardService.recordDetail(path)
      state.errors.detail = ''
    }, 'detail')
  }
  async function scan() {
    return wrap(async () => {
      const res = await dashboardService.refreshScan()
      await refreshAll()
      return res
    }, 'scan')
  }
  async function reconcileDB() {
    return wrap(async () => {
      const res = await dashboardService.reconcileDB()
      await refreshAll()
      return res
    }, 'reconcileDB')
  }
  async function start() {
    return wrap(async () => {
      const res = await dashboardService.startTasks({ mode: state.startMode, status: state.filters.status, search: state.filters.search })
      await refreshAll()
      return res
    }, 'start')
  }
  async function startCurrentFilter() {
    return wrap(async () => {
      const res = await dashboardService.startTasks({ mode: 'current_filter', status: state.filters.status, search: state.filters.search })
      await refreshAll()
      return res
    }, 'start')
  }
  async function startSelected() {
    return wrap(async () => {
      const res = await dashboardService.startSelectedTasks(state.selectedPaths)
      clearSelected()
      await refreshAll()
      return res
    }, 'start')
  }
  async function stopTasks() {
    return wrap(async () => {
      const res = await dashboardService.stopTasks()
      await refreshAll()
      return res
    }, 'stop')
  }
  async function retryFailed() {
    return wrap(async () => {
      const res = await dashboardService.retryFailedTasks()
      await refreshAll()
      return res
    }, 'retryFailed')
  }
  async function retrySelected() {
    return wrap(async () => {
      const res = await dashboardService.retrySelectedTasks({ paths: state.selectedPaths })
      clearSelected()
      await refreshAll()
      return res
    }, 'retryFailed')
  }
  async function retryByFilter() {
    return wrap(async () => {
      const res = await dashboardService.retrySelectedTasks({ status: state.filters.status, search: state.filters.search })
      await refreshAll()
      return res
    }, 'retryFailed')
  }
  async function retryOne(path) {
    state.loading.retryOne = path
    try {
      state.error = ''
      const res = await dashboardService.retryTask(path)
      await refreshAll()
      return res
    } catch (e) {
      state.error = e.message || String(e)
      throw e
    } finally {
      state.loading.retryOne = ''
    }
  }
  async function clearDB() {
    return wrap(async () => {
      const res = await dashboardService.clearDB()
      state.detail = null
      state.confirmClear = false
      clearSelected()
      await refreshAll()
      return res
    }, 'clearDB')
  }

  return {
    state,
    toggleSelected,
    clearSelected,
    toggleSelectAllCurrentPage,
    refreshOverview,
    refreshSettings,
    saveSettings,
    refreshRecords,
    refreshDownloaded,
    refreshCompleted,
    refreshAll,
    loadDetail,
    scan,
    reconcileDB,
    start,
    startCurrentFilter,
    startSelected,
    stopTasks,
    retryFailed,
    retrySelected,
    retryByFilter,
    retryOne,
    clearDB,
  }
}
