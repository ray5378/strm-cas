import { reactive } from '../vendor/vue.esm-browser.prod.js'

const state = reactive({ items: [] })
let id = 1

export function useToast() {
  function push(message, type = 'info', timeout = 2500) {
    const item = { id: id++, message, type }
    state.items.push(item)
    setTimeout(() => {
      const idx = state.items.findIndex(v => v.id === item.id)
      if (idx >= 0) state.items.splice(idx, 1)
    }, timeout)
  }
  return {
    items: state.items,
    success(message) { push(message, 'success') },
    error(message) { push(message, 'error', 4000) },
    info(message) { push(message, 'info') },
  }
}
