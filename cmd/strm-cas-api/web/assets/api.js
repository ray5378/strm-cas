export async function getJSON(url, options = {}) {
  const res = await fetch(url, options)
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`)
  return data
}
export const api = {
  overview: () => getJSON('/api/overview'),
  records: (params) => getJSON('/api/records?' + new URLSearchParams(params)),
  recordDetail: (path) => getJSON('/api/records/detail?' + new URLSearchParams({ path })),
  runtime: () => getJSON('/api/runtime'),
  runtimeDownloaded: (params) => getJSON('/api/runtime/downloaded?' + new URLSearchParams(params)),
  runtimeCompleted: (params) => getJSON('/api/runtime/completed?' + new URLSearchParams(params)),
  refreshScan: () => getJSON('/api/scan/refresh', { method: 'POST' }),
  startTasks: () => getJSON('/api/tasks/start', { method: 'POST' }),
  retryTask: (path) => getJSON('/api/tasks/retry', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path }) }),
  retryFailedTasks: () => getJSON('/api/tasks/retry-failed', { method: 'POST' }),
  clearDB: () => getJSON('/api/db/clear', { method: 'POST' }),
}
