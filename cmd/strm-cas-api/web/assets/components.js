export function statsCards(stats = {}) {
  const items = [
    ['总 .strm', stats.total || 0],
    ['还没处理', stats.unprocessed || 0],
    ['已生成 .cas', stats.done || 0],
    ['失败', stats.failed || 0],
    ['异常', stats.exception || 0],
    ['被跳过', stats.skipped || 0],
  ]
  return `<div class="grid">${items.map(([k,v]) => `<div class="card"><div class="muted">${k}</div><div style="font-size:30px;font-weight:700">${v}</div></div>`).join('')}</div>`
}

export function pager(total, page, pageSize, prefix) {
  const pages = Math.max(1, Math.ceil((total || 0) / pageSize))
  return `<div class="row"><button data-act="${prefix}:prev" ${page<=1?'disabled':''}>上一页</button><span class="muted">第 ${page} / ${pages} 页，共 ${total} 条</span><button data-act="${prefix}:next" ${page>=pages?'disabled':''}>下一页</button></div>`
}

export function statusBadge(status='') {
  return `<span class="badge ${status || 'pending'}">${status || 'pending'}</span>`
}
