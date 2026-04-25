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
    confirmClear: false,
  })

  async function wrap(action) {
    try {
      state.error = ''
      return await action()
    } catch (e) {
      state.error = e.message || String(e)
      throw e
    }
  }

  async function refreshOverview() { state.overview = await dashboardService.overview() }
  async function refreshRecords() { state.records = await dashboardService.records(state.filters) }
  async function refreshDownloaded() { state.downloaded = await dashboardService.runtimeDownloaded({ page: state.downloadedPage, page_size: 10 }) }
  async function refreshCompleted() { state.completed = await dashboardService.runtimeCompleted({ page: state.completedPage, page_size: 10, status: state.completedStatus }) }
  async function refreshAll() { return wrap(async () => { await Promise.all([refreshOverview(), refreshRecords(), refreshDownloaded(), refreshCompleted()]) }) }
  async function loadDetail(path) { return wrap(async () => { state.detail = await dashboardService.recordDetail(path) }) }
  async function scan() { return wrap(async () => { const res = await dashboardService.refreshScan(); await refreshAll(); return res }) }
  async function start() { return wrap(async () => { const res = await dashboardService.startTasks({ mode: state.startMode, status: state.filters.status, search: state.filters.search }); await refreshAll(); return res }) }
  async function retryFailed() { return wrap(async () => { const res = await dashboardService.retryFailedTasks(); await refreshAll(); return res }) }
  async function retryOne(path) { return wrap(async () => { const res = await dashboardService.retryTask(path); await refreshAll(); return res }) }
  async function clearDB() { return wrap(async () => { const res = await dashboardService.clearDB(); state.detail = null; state.confirmClear = false; await refreshAll(); return res }) }

  return { state, refreshOverview, refreshRecords, refreshDownloaded, refreshCompleted, refreshAll, loadDetail, scan, start, retryFailed, retryOne, clearDB }
}
