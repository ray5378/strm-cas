export const ToastStack = {
  props: { items: { type: Array, default: () => [] } },
  template: `
    <div class="toast-stack" v-if="items.length">
      <div v-for="item in items" :key="item.id" class="toast" :class="item.type || 'info'">
        {{ item.message }}
      </div>
    </div>
  `,
}
