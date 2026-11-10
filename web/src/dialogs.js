import { reactive } from 'vue'

export const dialogState = reactive({
  open: false,
  tone: 'danger',
  icon: 'Trash2',
  eyebrow: '请确认操作',
  title: '',
  message: '',
  details: [],
  note: '',
  confirmText: '确定',
  cancelText: '取消',
  resolve: null,
})

export function confirmDialog(options = {}) {
  if (dialogState.open) settleDialog(false)
  const tone = options.tone || 'danger'
  dialogState.tone = tone
  dialogState.icon = options.icon || (tone === 'danger' ? 'Trash2' : tone === 'warning' ? 'CircleAlert' : 'Info')
  dialogState.eyebrow = options.eyebrow || (tone === 'danger' ? '危险操作' : tone === 'warning' ? '请确认' : '提示')
  dialogState.title = options.title || '请确认操作'
  dialogState.message = options.message || ''
  dialogState.details = Array.isArray(options.details) ? options.details.filter(Boolean) : []
  dialogState.note = options.note || ''
  dialogState.confirmText = options.confirmText || '确定'
  dialogState.cancelText = options.cancelText || '取消'
  dialogState.open = true
  return new Promise(resolve => { dialogState.resolve = resolve })
}

export function settleDialog(result) {
  if (!dialogState.open) return
  const resolve = dialogState.resolve
  dialogState.open = false
  dialogState.resolve = null
  resolve?.(result)
}

export function confirmDelete({ title, message, details = [], note = '此操作不可撤销。', confirmText = '删除' }) {
  return confirmDialog({ tone: 'danger', icon: 'Trash2', eyebrow: '危险操作', title, message, details, note, confirmText, cancelText: '取消' })
}

export function confirmDiscard({ title = '放弃未保存的修改？', message, confirmText = '放弃修改', cancelText = '继续编辑' }) {
  return confirmDialog({ tone: 'warning', icon: 'CircleAlert', eyebrow: '未保存的修改', title, message, confirmText, cancelText })
}

export function detailList(values, limit = 4) {
  const items = (values || []).filter(value => value !== undefined && value !== null && value !== '')
  if (!items.length) return []
  if (items.length <= limit) return items
  return [...items.slice(0, limit), `以及其余 ${items.length - limit} 项`]
}
