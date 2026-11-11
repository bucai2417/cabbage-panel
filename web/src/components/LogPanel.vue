<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { api, usePolling, statusLabel, copy, date } from '../api'
import Icon from './Icon.vue'
const props = defineProps({ runId: Number, taskId: Number, kind: { type: String, default: 'crons' }, title: { type: String, default: '运行日志' }, embedded: Boolean, stoppable: Boolean })
const emit = defineEmits(['close', 'status', 'stop'])
const container = ref(null), previousFocus = document.activeElement
const content = ref(''), status = ref(''), runs = ref([]), activeId = ref(props.runId || 0), loading = ref(false), error = ref(''), query = ref(''), follow = ref(true), terminal = ref(null), lastHistory = ref(0), stdinValue = ref(''), stdinBusy = ref(false), stdinSentAt = ref(0)
const visibleContent = computed(() => query.value ? content.value.split('\n').filter(line => line.toLowerCase().includes(query.value.toLowerCase())).join('\n') : content.value)
const selectedRun = computed(() => runs.value.find(run => run.id === activeId.value))
async function loadHistory() { if (!props.taskId) return; runs.value = await api('/logs/runs?kind=' + props.kind + '&taskId=' + props.taskId + '&limit=500'); if (!activeId.value && runs.value.length) activeId.value = runs.value[0].id; lastHistory.value = Date.now() }
async function loadContent(force = false) { const id = activeId.value; if (!id || (!force && !['running', 'queued', ''].includes(status.value))) return; try { const result = await api('/logs/run/' + id); if (id !== activeId.value) return; content.value = result.content || (['queued', 'running'].includes(result.status) ? '任务等待执行或尚未输出…' : '本次执行没有日志输出。'); status.value = result.status; error.value = ''; if (follow.value && !window.getSelection()?.toString()) { await nextTick(); if (terminal.value) terminal.value.scrollTop = terminal.value.scrollHeight } } catch (failure) { if (id === activeId.value) error.value = failure.message } }
usePolling(async () => { if (props.taskId && Date.now() - lastHistory.value > 5000) await loadHistory(); await loadContent() }, 1500)
watch(activeId, async () => { content.value = ''; status.value = ''; loading.value = true; await loadContent(true); loading.value = false })
watch(() => props.runId, value => { if (value) activeId.value = value })
watch(status, value => emit('status', value))
// 检测日志末尾是否在等待输入（Python input()、Node readline、Shell read 等）。
// 最可靠的特征：提示符不换行，日志内容末尾没有换行符，且最后一行非空。
const waitingInput = computed(() => {
  if (!['running', ''].includes(status.value)) return false
  const raw = content.value || ''
  if (!raw) return false
  const trimmed = raw.replace(/\s+$/, '')
  if (!trimmed) return false
  const lastLine = trimmed.slice(trimmed.lastIndexOf('\n') + 1).trim()
  if (!lastLine) return false
  // 提示符未换行 → 程序停在输入等待；或最后一行形如提示（冒号/问号/大于号/「请输入」）。
  const noTrailingNewline = !raw.endsWith('\n')
  const promptLike = /[:：?？>]\s*$/.test(lastLine) || /^(请输入|input|enter|password|username)/i.test(lastLine)
  return noTrailingNewline || promptLike
})
async function sendInput() {
  if (stdinBusy.value || !activeId.value) return
  const value = stdinValue.value
  stdinBusy.value = true
  try {
    await api('/logs/input', 'POST', { id: activeId.value, input: value })
    stdinValue.value = ''
    stdinSentAt.value = Date.now()
    // 连续快速刷新，让输入回显和后续输出尽快出现。
    await loadContent(true)
    for (let i = 0; i < 4; i++) { await new Promise(r => setTimeout(r, 250)); await loadContent(true) }
  } catch (failure) { error.value = failure.message } finally { stdinBusy.value = false }
}
function keyboard(event) {
  if (props.embedded || event.defaultPrevented) return
  if (event.key === 'Escape') { event.preventDefault(); emit('close') }
  if (event.key === 'Tab') {
    const elements = [...(container.value?.querySelectorAll('button:not(:disabled), input, select, a, [tabindex="0"]') || [])].filter(element => element.offsetParent !== null)
    if (event.shiftKey && document.activeElement === elements[0]) { event.preventDefault(); elements.at(-1)?.focus() }
    else if (!event.shiftKey && document.activeElement === elements.at(-1)) { event.preventDefault(); elements[0]?.focus() }
  }
}
onMounted(() => { window.addEventListener('keydown', keyboard); if (!props.embedded) container.value?.querySelector('select, button')?.focus() })
onBeforeUnmount(() => { window.removeEventListener('keydown', keyboard); if (!props.embedded) previousFocus?.focus() })
</script>
<template>
  <div :class="embedded ? 'log-embedded' : 'overlay log-overlay'" @mousedown.self="!embedded && $emit('close')"><section ref="container" :class="embedded ? 'terminal-wrap' : 'modal log-modal'" :role="embedded ? 'region' : 'dialog'" :aria-modal="!embedded" :aria-label="title">
    <header v-if="!embedded"><div class="actions"><button class="icon-button" aria-label="关闭日志" @click="$emit('close')"><Icon name="X" /></button></div></header>
    <div class="log-controls"><label v-if="taskId" class="log-history-select">历史记录<select v-model="activeId" aria-label="选择历史日志"><option v-if="!runs.length" :value="0">暂无运行记录</option><option v-for="run in runs" :key="run.id" :value="run.id">#{{ run.id }} · {{ date(run.started || run.created) }} · {{ statusLabel[run.status] }} · {{ run.duration?.toFixed(2) || '0' }}s</option></select></label><input v-model="query" aria-label="搜索日志内容" placeholder="筛选日志行…" /><label class="inline-check"><input v-model="follow" type="checkbox" />跟随输出</label><a v-if="activeId" class="small-button" :href="'/api/logs/download/' + activeId" download>下载完整日志</a></div>
    <div class="terminal-shell"><pre ref="terminal" class="terminal" tabindex="0">{{ error || (loading ? '正在加载…' : visibleContent || (query ? '没有匹配的日志行' : '暂无运行记录')) }}</pre><button class="terminal-copy" aria-label="复制日志" title="复制日志" @click="copy(content)"><Icon name="Copy" :size="15" /></button></div>
    <form v-if="waitingInput" class="stdin-bar" @submit.prevent="sendInput"><span class="stdin-hint"><Icon name="Terminal" :size="15" />程序等待输入</span><input v-model="stdinValue" class="stdin-input" placeholder="输入内容后回车发送…" aria-label="标准输入" autocomplete="off" spellcheck="false" :disabled="stdinBusy" /><button class="button primary compact" :disabled="stdinBusy"><Icon :name="stdinBusy ? 'LoaderCircle' : 'Send'" :size="15" :class="{ spin: stdinBusy }" />发送</button></form>
    <footer><span class="muted">{{ selectedRun ? '退出码 ' + (selectedRun.exitCode ?? '—') + ' · ' : '' }}UTF-8 · 运行中自动刷新</span><span class="muted">预览末尾 2 MB · 历史最多 500 次</span></footer>
  </section></div>
</template>
