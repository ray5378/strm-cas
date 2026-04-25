import { reactive } from '../vendor/vue.esm-browser.prod.js'
import { dashboardService } from '../services/dashboardService.js'

export function useDashboardStore() {
  const state = reactive({
    overview: null,
    records: { total: 0, items: [] },
    downloaded: { total: 0, items: [] },
    completed: { total: 0, items: [] },
    detail: null,
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
      retryFailed: false,
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

  async function refreshOverview() {
    try {
      state.overview = await dashboardService.overview()
      state.errors.overview = ''
    } catch (e) {
      state.errors.overview = e.message || String(e)
      throw e
    }
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
  async function scan() { return wrap(async () => { const res = await dashboardService.refreshScan(); await refreshAll(); return res }, 'scan') }
  async function start() { return wrap(async () => { const res = await dashboardService.startTasks({ mode: state.startMode, status: state.filters.status, search: state.filters.search }); await refreshAll(); return res }, 'start') }
  async function retryFailed() { return wrap(async () => { const res = await dashboardService.retryFailedTasks(); await refreshAll(); return res }, 'retryFailed') }
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
  async function clearDB() { return wrap(async () => { const res = await dashboardService.clearDB(); state.detail = null; state.confirmClear = false; await refreshAll(); return res }, 'clearDB') }

  return { state, refreshOverview, refreshRecords, refreshDownloaded, refreshCompleted, refreshAll, loadDetail, scan, start, retryFailed, retryOne, clearDB }
}
