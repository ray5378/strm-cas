import { api } from './api.js'

export function createStore() {
  return {
    overview: null,
    records: { total: 0, items: [] },
    downloaded: { total: 0, items: [] },
    completed: { total: 0, items: [] },
    detail: null,
    filters: { status: '', search: '', page: 1, page_size: 10 },
    downloadedPage: 1,
    completedPage: 1,
    completedStatus: '',
    recordsJump: '',
    downloadedJump: '',
    completedJump: '',
    startMode: 'pending',
    error: '',
    async refreshOverview() { this.overview = await api.overview() },
    async refreshRecords() { this.records = await api.records(this.filters) },
    async refreshDownloaded() { this.downloaded = await api.runtimeDownloaded({ page: this.downloadedPage, page_size: 10 }) },
    async refreshCompleted() { this.completed = await api.runtimeCompleted({ page: this.completedPage, page_size: 10, status: this.completedStatus }) },
    async loadDetail(path) { this.detail = await api.recordDetail(path) },
    async refreshScan() { return await api.refreshScan() },
    async startTasks() { return await api.startTasks({ mode: this.startMode, status: this.filters.status, search: this.filters.search }) },
    async retryFailedTasks() { return await api.retryFailedTasks() },
    async refreshAll() {
      try {
        this.error = ''
        await Promise.all([this.refreshOverview(), this.refreshRecords(), this.refreshDownloaded(), this.refreshCompleted()])
      } catch (e) { this.error = e.message || String(e) }
    }
  }
}
