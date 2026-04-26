import { getJSON, postJSON } from '../api/client.js'

export const dashboardService = {
  saveSettings: (payload) => postJSON('/api/settings', payload || {}),
  refreshScan: () => postJSON('/api/scan/refresh'),
  reconcileDB: () => postJSON('/api/db/reconcile'),
  startTasks: (payload) => postJSON('/api/tasks/start', payload || {}),
  startSelectedTasks: (paths) => postJSON('/api/tasks/start-selected', { paths }),
  stopTasks: () => postJSON('/api/tasks/stop'),
  stopAfterCurrentTasks: () => postJSON('/api/tasks/stop-after-current'),
  retryTask: (path) => postJSON('/api/tasks/retry', { path }),
  retryFailedTasks: () => postJSON('/api/tasks/retry-failed'),
  retrySelectedTasks: (payload) => postJSON('/api/tasks/retry-selected', payload || {}),
  clearDB: () => postJSON('/api/db/clear'),
}
