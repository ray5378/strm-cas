import { getJSON, postJSON } from '../api/client.js'

export const dashboardService = {
  overview: () => getJSON('/api/overview'),
  settings: () => getJSON('/api/settings'),
  saveSettings: (payload) => postJSON('/api/settings', payload || {}),
  records: (params) => getJSON('/api/records?' + new URLSearchParams(params)),
  recordDetail: (path) => getJSON('/api/records/detail?' + new URLSearchParams({ path })),
  runtime: () => getJSON('/api/runtime'),
  runtimeDownloaded: (params) => getJSON('/api/runtime/downloaded?' + new URLSearchParams(params)),
  runtimeCompleted: (params) => getJSON('/api/runtime/completed?' + new URLSearchParams(params)),
  refreshScan: () => postJSON('/api/scan/refresh'),
  reconcileDB: () => postJSON('/api/db/reconcile'),
  startTasks: (payload) => postJSON('/api/tasks/start', payload || {}),
  startSelectedTasks: (paths) => postJSON('/api/tasks/start-selected', { paths }),
  stopTasks: () => postJSON('/api/tasks/stop'),
  retryTask: (path) => postJSON('/api/tasks/retry', { path }),
  retryFailedTasks: () => postJSON('/api/tasks/retry-failed'),
  retrySelectedTasks: (payload) => postJSON('/api/tasks/retry-selected', payload || {}),
  clearDB: () => postJSON('/api/db/clear'),
}
