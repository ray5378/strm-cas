import { computed } from '../vendor/vue.esm-browser.prod.js'
import { pages } from './ui.js'

export const PagerControl = {
  props: {
    total: { type: Number, default: 0 },
    page: { type: Number, default: 1 },
    pageSize: { type: Number, default: 10 },
  },
  emits: ['prev', 'next', 'jump'],
  setup(props, { emit }) {
    const totalPages = computed(() => pages(props.total, props.pageSize))
    const doJump = (e) => {
      const form = e.target.closest('form')
      const value = Number(form?.page?.value || 1)
      if (value > 0) emit('jump', value)
    }
    return { totalPages, doJump, emit }
  },
  template: `
    <div class="pager-wrap">
      <div class="row">
        <button @click="emit('prev')" :disabled="page <= 1">上一页</button>
        <span class="muted">第 {{ page }} / {{ totalPages }} 页，共 {{ total }} 条</span>
        <button @click="emit('next')" :disabled="page >= totalPages">下一页</button>
      </div>
      <form class="row" @submit.prevent="doJump">
        <input name="page" type="number" min="1" placeholder="页码" style="width:90px" />
        <button type="submit">跳转</button>
      </form>
    </div>
  `,
}
