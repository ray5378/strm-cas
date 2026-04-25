import { reactive } from '../vendor/vue.esm-browser.prod.js'
import { api } from '../api.js'

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
    try { state.error = ''; return await action() } catch (e) { state.error = e.message || String(e); throw e }
  }
  async function refreshOverview() { state.overview = await api.overview() }
  async function refreshRecords() { state.records = await api.records(state.filters) }
  async function refreshDownloaded() { state.downloaded = await api.runtimeDownloaded({ page: state.downloadedPage, page_size: 10 }) }
  async function refreshCompleted() { state.completed = await api.runtimeCompleted({ page: state.completedPage, page_size: 10, status: state.completedStatus }) }
  async function refreshAll() { return wrap(async () => { await Promise.all([refreshOverview(), refreshRecords(), refreshDownloaded(), refreshCompleted()]) }) }
  async function loadDetail(path) { return wrap(async () => { state.detail = await api.recordDetail(path) }) }
  async function scan() { return wrap(async () => { await api.refreshScan(); await refreshAll() }) }
  async function start() { return wrap(async () => { await api.startTasks({ mode: state.startMode, status: state.filters.status, search: state.filters.search }); await refreshAll() }) }
  async function retryFailed() { return wrap(async () => { await api.retryFailedTasks(); await refreshAll() }) }
  async function retryOne(path) { return wrap(async () => { await api.retryTask(path); await refreshAll() }) }
  async function clearDB() { return wrap(async () => { await api.clearDB(); state.detail = null; state.confirmClear = false; await refreshAll() }) }

  return { state, refreshOverview, refreshRecords, refreshDownloaded, refreshCompleted, refreshAll, loadDetail, scan, start, retryFailed, retryOne, clearDB }
}
