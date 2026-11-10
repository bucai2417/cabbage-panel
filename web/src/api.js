import { ref, onMounted, onUnmounted } from 'vue'

export const notices = ref([])
export function toast(message, type = 'success') {
  const id = Date.now() + Math.random()
  notices.value = [...notices.value.slice(-2), { id, message, type }]
  setTimeout(() => { notices.value = notices.value.filter(item => item.id !== id) }, 5000)
}
export async function api(path, method = 'GET', body) {
  const form = body instanceof FormData
  const response = await fetch(path.startsWith('/open') ? path : '/api' + path, {
    method, credentials: 'same-origin',
    headers: body && !form ? { 'Content-Type': 'application/json' } : {},
    body: body ? (form ? body : JSON.stringify(body)) : undefined,
  })
  let result
  try { result = await response.json() } catch { throw new Error('服务返回异常，请检查连接') }
  if (!response.ok || result.code !== 200) {
    if (response.status === 401 && path !== '/user/login') window.dispatchEvent(new Event('session-expired'))
    throw new Error(result.message || '请求失败')
  }
  return result.data
}
export async function action(callback, message) {
  try { const result = await callback(); if (message) toast(message); return result }
  catch (error) { toast(error.message, 'error'); return undefined }
}
export function usePolling(callback, interval = 5000) {
  let timer, active = true, busy = false
  const update = async () => {
    if (!active || busy) return
    if (document.hidden) { timer = setTimeout(update, interval); return }
    busy = true
    try { await callback() } catch (error) { if (!timer) toast(error.message, 'error') }
    finally { busy = false; if (active) timer = setTimeout(update, interval) }
  }
  onMounted(update)
  onUnmounted(() => { active = false; clearTimeout(timer) })
  return update
}
export function date(value) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—' }
export function bytes(value) { if (!value) return '0 B'; const index = Math.min(3, Math.floor(Math.log(value) / Math.log(1024))); return (value / 1024 ** index).toFixed(index ? 1 : 0) + ' ' + ['B', 'KB', 'MB', 'GB'][index] }
export const statusLabel = { success: '成功', failed: '失败', timeout: '超时', interrupted: '中断', cancelled: '已停止', running: '运行中', queued: '排队中' }
export async function copy(value) { try { await navigator.clipboard.writeText(value); toast('已复制') } catch { toast('浏览器不允许复制，请手动选择文本', 'error') } }
