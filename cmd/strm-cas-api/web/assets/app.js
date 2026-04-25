import { createApp } from './vue.js'
import { createStore } from './store.js'
import { statsCards, pager, statusBadge, stageText } from './components.js'
import { api } from './api.js'

const store = createStore()
let confirmClear = false

const root = {
  render() {
    const ov = store.overview || { stats: {}, runtime: {} }
    const current = ov.runtime?.current
    return `
      <div class="layout">
        <div class="title">strm-cas 控制台</div>
        ${store.error ? `<div class="card" style="background:#fee2e2;color:#991b1b">${store.error}</div>` : ''}
        ${statsCards(ov.stats)}
        <div class="section card">
          <div class="toolbar">
            <button data-act="scan" ${ov.runtime?.running ? 'disabled' : ''}>开始扫描 /strm</button>
            <button data-act="refresh">刷新</button>
            ${!confirmClear ? `<button data-act="clear-db-step1" ${ov.runtime?.running ? 'disabled' : ''} style="background:#dc2626">清理数据库</button>` : `<button data-act="clear-db-step2" ${ov.runtime?.running ? 'disabled' : ''} style="background:#b91c1c">确认清理数据库</button><button data-act="clear-db-cancel" style="background:#64748b">取消</button>`}
            <span class="muted">运行中: ${ov.runtime?.running ? '是' : '否'}</span>
            <span class="muted">已下载: ${ov.runtime?.downloaded_count || 0}</span>
            <span class="muted">已完成: ${ov.runtime?.completed_count || 0}</span>
          </div>
          ${confirmClear ? `<div class="section" style="color:#991b1b"><strong>二级确认：</strong>清理数据库会删除当前所有处理状态记录，但不会删除 /strm、/download 里的文件。</div>` : ''}
          <div class="section">
            <div><strong>当前任务</strong></div>
            ${current ? `<div class="mono">${current.job?.strm_path || ''}</div><div class="row"><span>${stageText(current.stage)}</span><span>${current.file_name || ''}</span><span>${current.downloaded_bytes || 0}${current.total_bytes ? ' / ' + current.total_bytes : ''}</span></div><div class="muted">${current.message || ''}</div>` : '<div class="muted">暂无</div>'}
          </div>
        </div>
        <div class="main-grid section">
          <div>
            <div class="card">
              <div class="toolbar">
                <strong>数据库记录</strong>
                <select id="status"><option value="">全部</option><option value="pending">未处理</option><option value="done">已完成</option><option value="failed">失败</option><option value="exception">异常</option><option value="skipped">已跳过</option></select>
                <input id="search" placeholder="搜索路径 / URL / 错误" value="${escapeHtml(store.filters.search)}" />
                <button data-act="apply-filters">筛选</button>
              </div>
              <table><thead><tr><th>状态</th><th>strm</th><th>cas</th><th>最后结果</th><th></th></tr></thead><tbody>
                ${(store.records.items || []).map(it => `<tr><td>${statusBadge(it.status)}</td><td class="mono">${escapeHtml(it.strm_path)}</td><td class="mono">${escapeHtml(it.cas_path || '')}</td><td>${escapeHtml(it.last_message || '')}</td><td><button data-detail="${encodeURIComponent(it.strm_path)}">详情</button></td></tr>`).join('') || '<tr><td colspan="5" class="muted">无数据</td></tr>'}
              </tbody></table>
              ${pager(store.records.total, store.filters.page, store.filters.page_size, 'records')}
            </div>
            <div class="card section">
              <strong>已下载任务</strong>
              <table><thead><tr><th>阶段</th><th>文件</th><th>下载路径</th><th>更新时间</th></tr></thead><tbody>
                ${(store.downloaded.items || []).map(it => `<tr><td>${escapeHtml(stageText(it.stage))}</td><td>${escapeHtml(it.file_name || '')}</td><td class="mono">${escapeHtml(it.download_path || '')}</td><td>${escapeHtml(it.updated_at || '')}</td></tr>`).join('') || '<tr><td colspan="4" class="muted">无数据</td></tr>'}
              </tbody></table>
              ${pager(store.downloaded.total, store.downloadedPage, 10, 'downloaded')}
            </div>
            <div class="card section">
              <div class="toolbar"><strong>已完成任务</strong><select id="completedStatus"><option value="">全部</option><option value="done">已完成</option><option value="failed">失败</option><option value="exception">异常</option><option value="skipped">已跳过</option></select><button data-act="apply-completed">筛选</button></div>
              <table><thead><tr><th>状态</th><th>strm</th><th>cas</th><th>消息</th></tr></thead><tbody>
                ${(store.completed.items || []).map(it => `<tr><td>${statusBadge(it.status)}</td><td class="mono">${escapeHtml(it.job?.strm_path || '')}</td><td class="mono">${escapeHtml(it.cas_path || '')}</td><td>${escapeHtml(it.message || '')}</td></tr>`).join('') || '<tr><td colspan="4" class="muted">无数据</td></tr>'}
              </tbody></table>
              ${pager(store.completed.total, store.completedPage, 10, 'completed')}
            </div>
          </div>
          <div>
            <div class="card"><strong>详情</strong>${store.detail ? `<div class="section mono">${renderDetail(store.detail)}</div>` : '<div class="muted section">点击“详情”查看</div>'}</div>
          </div>
        </div>
      </div>`
  },
  bind(el) {
    el.onclick = async (e) => {
      const act = e.target?.dataset?.act
      const detail = e.target?.dataset?.detail
      try {
        if (detail) { await store.loadDetail(decodeURIComponent(detail)); rerender() }
        if (retry) { await api.retryTask(decodeURIComponent(retry)); await store.refreshAll(); rerender() }
        if (act === 'scan') { await store.startScan(); await store.refreshAll(); rerender() }
        if (act === 'refresh') { await store.refreshAll(); rerender() }
        if (act === 'clear-db-step1') { confirmClear = true; rerender() }
        if (act === 'clear-db-cancel') { confirmClear = false; rerender() }
        if (act === 'clear-db-step2') { await api.clearDB(); confirmClear = false; await store.refreshAll(); rerender() }
        if (act === 'apply-filters') { store.filters.status = el.querySelector('#status').value; store.filters.search = el.querySelector('#search').value; store.filters.page = 1; await store.refreshRecords(); rerender() }
        if (act === 'apply-completed') { store.completedStatus = el.querySelector('#completedStatus').value; store.completedPage = 1; await store.refreshCompleted(); rerender() }
        if (act === 'records:prev') { if (store.filters.page > 1) store.filters.page--; await store.refreshRecords(); rerender() }
        if (act === 'records:next') { store.filters.page++; await store.refreshRecords(); rerender() }
        if (act === 'downloaded:prev') { if (store.downloadedPage > 1) store.downloadedPage--; await store.refreshDownloaded(); rerender() }
        if (act === 'downloaded:next') { store.downloadedPage++; await store.refreshDownloaded(); rerender() }
        if (act === 'completed:prev') { if (store.completedPage > 1) store.completedPage--; await store.refreshCompleted(); rerender() }
        if (act === 'completed:next') { store.completedPage++; await store.refreshCompleted(); rerender() }
      } catch (err) { store.error = err.message || String(err); rerender() }
    }
  },
  async mounted() {
    await store.refreshAll(); rerender(); setInterval(async () => { await store.refreshAll(); rerender() }, 3000)
  }
}

function rerender() { document.querySelector('#app').innerHTML = root.render(); root.bind(document.querySelector('#app')) }
function renderDetail(obj) { return Object.entries(obj).map(([k,v]) => `<div><strong>${escapeHtml(k)}:</strong> ${escapeHtml(typeof v === 'string' ? v : JSON.stringify(v, null, 2))}</div>`).join('') }
function escapeHtml(s='') { return String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;') }

createApp(root).mount('#app')
t;') }

createApp(root).mount('#app')
