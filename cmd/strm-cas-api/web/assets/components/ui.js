export const STATUS_TEXT = {
  pending: '未处理',
  done: '已完成',
  failed: '失败',
  exception: '异常',
  skipped: '已跳过',
}

export const STAGE_TEXT = {
  queued: '排队中',
  downloading: '下载中',
  downloaded: '已下载',
  generating_cas: '生成 CAS',
  completed: '已完成',
  cache_recovered: '缓存恢复',
}

export function statusText(status = '') {
  return STATUS_TEXT[status] || status || '未处理'
}

export function stageText(stage = '') {
  return STAGE_TEXT[stage] || stage || '-'
}

export function pages(total = 0, pageSize = 10) {
  return Math.max(1, Math.ceil((total || 0) / pageSize))
}
