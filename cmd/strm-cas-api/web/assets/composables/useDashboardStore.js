import { reactive } from '../vendor/vue.esm-browser.prod.js'
import { dashboardService } from '../services/dashboardService.js'

function wsURL(path) {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}${path}`
}

export function useDashboardStore() {
  const state = reactive({
    overview: null,
    settings: { concurrency: 2, total_rate_limit_mb: 0, max_file_size_gb: 0 },
    records: { total: 0, items: [] },
    detail: null,
    reconcileSummary: null,
    selectedPaths: [],
    filters: { status: '', search: '', page: 1, page_size: 10 },
    startMode: 'pending',
    error: '',
    errors: { records: '', detail: '', overview: '' },
    confirmClear: false,
    autoRefreshEnabled: true,
    runtimeSocketConnected: false,
    runtimeSocketError: '',
    loading: {
      initial: false,
      overview: false,
      refreshAll: false,
      scan: false,
      start: false,
      stop: false,
      stopAfterCurrent: false,
      saveSettings: false,
      retryFailed: false,
      reconcileDB: false,
      renameCAS: false,
      clearDB: false,
      records: false,
      detail: false,
      retryOne: '',
    },
  })

  let runtimeSocket = null
  let runtimeReconnectTimer = null
  let runtimeSocketClosedByUser = false

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

  function clearSelected() {
    state.selectedPaths.splice(0, state.selectedPaths.length)
  }

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
    for (const path of valid) {
      if (!state.selectedPaths.includes(path)) state.selectedPaths.push(path)
    }
  }

  function applyOverviewData(data) {
    state.overview = data
    state.errors.overview = ''
    if (state.overview?.settings) {
      state.settings = {
        concurrency: state.overview.settings.concurrency || 1,
        total_rate_limit_mb: state.overview.settings.total_rate_limit_mb || 0,
        max_file_size_gb: state.overview.settings.max_file_size_gb || 0,
      }
    }
  }

  function clearReadLoading() {
    state.loading.initial = false
    state.loading.overview = false
    state.loading.records = false
    state.loading.detail = false
    state.loading.refreshAll = false
  }

  function clearOverviewLoading() {
    state.loading.initial = false
    state.loading.overview = false
  }

  function markReadLoading() {
    state.loading.initial = true
    state.loading.overview = true
    state.loading.records = true
    if (state.detail?.strm_path) state.loading.detail = true
  }

  function applyDashboardSnapshot(payload) {
    if (payload?.overview) applyOverviewData(payload.overview)
    if (payload?.records) {
      state.records = payload.records
      state.errors.records = ''
      state.loading.records = false
    }
    if (payload && 'detail' in payload) {
      state.detail = payload.detail || null
      state.errors.detail = ''
      state.loading.detail = false
    }
    state.loading.refreshAll = false
  }

  function applyOverviewSnapshot(payload) {
    if (payload?.overview) applyOverviewData(payload.overview)
    clearOverviewLoading()
  }

  function applyRuntimeSnapshot(runtime) {
    const prev = state.overview || {}
    state.overview = { ...prev, runtime: runtime || {} }
  }

  function requestDashboardSnapshot() {
    if (!runtimeSocket || runtimeSocket.readyState !== WebSocket.OPEN) return false
    markReadLoading()
    state.errors.overview = ''
    state.errors.records = ''
    state.errors.detail = ''
    runtimeSocket.send(JSON.stringify({
      type: 'subscribe',
      status: state.filters.status || '',
      search: state.filters.search || '',
      page: state.filters.page || 1,
      page_size: state.filters.page_size || 10,
      detail_path: state.detail?.strm_path || '',
    }))
    return true
  }

  function connectRuntimeSocket() {
    runtimeSocketClosedByUser = false
    if (runtimeSocket) {
      try { runtimeSocket.close() } catch {}
      runtimeSocket = null
    }
    const ws = new WebSocket(wsURL('/api/runtime/ws'))
    runtimeSocket = ws
    ws.onopen = () => {
      state.runtimeSocketConnected = true
      state.runtimeSocketError = ''
      requestDashboardSnapshot()
    }
    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data?.type === 'overview') {
          applyOverviewSnapshot(data)
        } else if (data?.type === 'dashboard') {
          applyDashboardSnapshot(data)
        } else if (data?.type === 'runtime') {
          applyRuntimeSnapshot(data.runtime || {})
        }
      } catch (e) {
        state.runtimeSocketError = e.message || String(e)
      }
    }
    ws.onerror = () => {
      state.runtimeSocketError = 'websocket 连接异常'
      state.errors.overview = state.errors.overview || state.runtimeSocketError
      state.errors.records = state.errors.records || state.runtimeSocketError
      if (state.detail?.strm_path) state.errors.detail = state.errors.detail || state.runtimeSocketError
    }
    ws.onclose = () => {
      state.runtimeSocketConnected = false
      runtimeSocket = null
      if (runtimeSocketClosedByUser || !state.autoRefreshEnabled) return
      clearTimeout(runtimeReconnectTimer)
      runtimeReconnectTimer = setTimeout(() => {
        connectRuntimeSocket()
      }, 2000)
    }
  }

  function disconnectRuntimeSocket() {
    runtimeSocketClosedByUser = true
    state.runtimeSocketConnected = false
    clearTimeout(runtimeReconnectTimer)
    runtimeReconnectTimer = null
    clearReadLoading()
    if (runtimeSocket) {
      try { runtimeSocket.close() } catch {}
      runtimeSocket = null
    }
  }

  async function refreshOverview() {
    return wrap(async () => {
      requestDashboardSnapshot()
    }, null)
  }

  async function refreshSettings() {
    return wrap(async () => {
      requestDashboardSnapshot()
    }, null)
  }

  async function saveSettings() {
    return wrap(async () => {
      const s = await dashboardService.saveSettings(state.settings)
      state.settings = {
        concurrency: s.concurrency || 1,
        total_rate_limit_mb: s.total_rate_limit_mb || 0,
        max_file_size_gb: s.max_file_size_gb || 0,
      }
      requestDashboardSnapshot()
      return s
    }, 'saveSettings')
  }

  async function refreshRecords() {
    return wrap(async () => {
      requestDashboardSnapshot()
    }, null)
  }

  async function refreshAll() {
    return wrap(async () => {
      state.loading.refreshAll = true
      requestDashboardSnapshot()
    }, null)
  }

  async function loadDetail(path) {
    return wrap(async () => {
      state.detail = state.detail?.strm_path === path ? state.detail : { strm_path: path }
      requestDashboardSnapshot()
    }, null)
  }

  async function scan() {
    return wrap(async () => {
      const res = await dashboardService.refreshScan()
      requestDashboardSnapshot()
      return res
    }, 'scan')
  }

  async function reconcileDB() {
    return wrap(async () => {
      const res = await dashboardService.reconcileDB()
      state.reconcileSummary = res || null
      requestDashboardSnapshot()
      return res
    }, 'reconcileDB')
  }

  async function renameCAS() {
    return wrap(async () => {
      const res = await dashboardService.renameDecodedCAS()
      requestDashboardSnapshot()
      return res
    }, 'renameCAS')
  }

  async function start() {
    return wrap(async () => {
      const res = await dashboardService.startTasks({ mode: state.startMode, status: state.filters.status, search: state.filters.search })
      requestDashboardSnapshot()
      return res
    }, 'start')
  }

  async function startCurrentFilter() {
    return wrap(async () => {
      const res = await dashboardService.startTasks({ mode: 'current_filter', status: state.filters.status, search: state.filters.search })
      requestDashboardSnapshot()
      return res
    }, 'start')
  }

  async function startSelected() {
    return wrap(async () => {
      const res = await dashboardService.startSelectedTasks(state.selectedPaths)
      clearSelected()
      requestDashboardSnapshot()
      return res
    }, 'start')
  }

  async function stopTasks() {
    return wrap(async () => {
      const res = await dashboardService.stopTasks()
      requestDashboardSnapshot()
      return res
    }, 'stop')
  }

  async function stopAfterCurrentTasks() {
    return wrap(async () => {
      const res = await dashboardService.stopAfterCurrentTasks()
      requestDashboardSnapshot()
      return res
    }, 'stopAfterCurrent')
  }

  async function retryFailed() {
    return wrap(async () => {
      const res = await dashboardService.retryFailedTasks()
      requestDashboardSnapshot()
      return res
    }, 'retryFailed')
  }

  async function retrySelected() {
    return wrap(async () => {
      const res = await dashboardService.retrySelectedTasks({ paths: state.selectedPaths })
      clearSelected()
      requestDashboardSnapshot()
      return res
    }, 'retryFailed')
  }

  async function retryByFilter() {
    return wrap(async () => {
      const res = await dashboardService.retrySelectedTasks({ status: state.filters.status, search: state.filters.search })
      requestDashboardSnapshot()
      return res
    }, 'retryFailed')
  }

  async function retryOne(path) {
    state.loading.retryOne = path
    try {
      state.error = ''
      const res = await dashboardService.retryTask(path)
      requestDashboardSnapshot()
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
      requestDashboardSnapshot()
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
    refreshAll,
    loadDetail,
    scan,
    reconcileDB,
    renameCAS,
    start,
    startCurrentFilter,
    startSelected,
    stopTasks,
    stopAfterCurrentTasks,
    retryFailed,
    retrySelected,
    retryByFilter,
    retryOne,
    clearDB,
    connectRuntimeSocket,
    disconnectRuntimeSocket,
  }
}
