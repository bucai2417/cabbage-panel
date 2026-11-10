<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount, defineAsyncComponent } from 'vue'
import { api, toast } from '../api'
import { confirmDiscard } from '../dialogs'
import Icon from './Icon.vue'
import LogPanel from './LogPanel.vue'
import EnvironmentDialog from './EnvironmentDialog.vue'
const CodeEditor = defineAsyncComponent(() => import('./CodeEditor.vue'))

const props = defineProps({ path: String, content: { type: String, default: '' } })
const emit = defineEmits(['close', 'saved'])
const code = ref(props.content), base = ref(props.content), editor = ref(null)
const runId = ref(0), runStatus = ref(''), runError = ref(''), runSummary = ref('')
const running = ref(false), stopping = ref(false), saving = ref(false), envOpen = ref(false)
const diagnostics = ref([])
const dirty = computed(() => code.value !== base.value)
const isRunning = computed(() => ['running', 'queued'].includes(runStatus.value))
function clearDiagnostics() { diagnostics.value = []; runSummary.value = ''; editor.value?.setDiagnostics([]) }
async function save() {
  if (saving.value) return
  saving.value = true
  try {
    await api('/scripts', 'PUT', { filename: props.path, content: code.value })
    base.value = code.value
    emit('saved', code.value)
    toast('脚本已保存')
  } catch (error) { toast(error.message, 'error') } finally { saving.value = false }
}
async function run() {
  if (running.value) return
  runError.value = ''
  clearDiagnostics()
  runId.value = 0
  runStatus.value = 'queued'
  running.value = true
  try {
    const result = await api('/scripts/debug', 'POST', { filename: props.path, content: code.value })
    runId.value = result.runId
  } catch (error) {
    runError.value = error.message
    runStatus.value = ''
    toast(error.message, 'error')
  } finally { running.value = false }
}
async function stopRun() {
  if (stopping.value) return
  stopping.value = true
  try {
    await api('/scripts/stop', 'PUT', { filename: props.path })
    toast('已请求停止脚本运行')
  } catch (error) { toast(error.message, 'error') } finally { stopping.value = false }
}
function parseDiagnostics(text) {
  const lines = String(text || '').split('\n').map(line => line.trimEnd())
  if (!lines.length) return []
  let jsLine = 0
  for (const raw of lines) {
    if (/^\s*at\s/.test(raw)) {
      const match = raw.match(/:(\d+):\d+/)
      if (match) { jsLine = Number(match[1]); break }
    }
  }
  const lineNumbers = []
  for (const raw of lines) {
    const match = raw.match(/line\s+(\d+)/i)
    if (match) lineNumbers.push(Number(match[1]))
  }
  const summary = [...lines].reverse().find(line => {
    const trimmed = line.trim()
    return trimmed && !/^(错误:|结束状态:)/.test(trimmed) && /(error|exception|错误|failed|not found|Traceback|SyntaxError|NameError|TypeError|ValueError|ReferenceError)/i.test(trimmed)
  })
  const fallback = [...lines].reverse().find(line => line.trim() && !/^结束状态:/.test(line.trim()))
  const message = (summary || fallback || '运行失败').trim().slice(0, 500)
  const line = jsLine || (lineNumbers.length ? lineNumbers[lineNumbers.length - 1] : 1)
  return [{ line, message }]
}
async function loadDiagnostics() {
  const id = runId.value
  if (!id) return
  try {
    const result = await api('/logs/run/' + id)
    if (id !== runId.value) return
    const items = parseDiagnostics(result.content)
    diagnostics.value = items
    runSummary.value = items[0] ? (items[0].line ? '第 ' + items[0].line + ' 行 · ' : '') + items[0].message : ''
    editor.value?.setDiagnostics(items)
  } catch { /* 保留日志本身即可 */ }
}
function onRunStatus(value) {
  runStatus.value = value
  if (['failed', 'timeout', 'interrupted'].includes(value)) loadDiagnostics()
  if (value === 'success' || value === 'cancelled') clearDiagnostics()
}
async function close() {
  if (dirty.value && !await confirmDiscard({
    title: '放弃调试修改？',
    message: '调试工作区有未保存的代码。退出不会保存，也不会把这些修改带回脚本编辑器。',
    confirmText: '退出并放弃',
    cancelText: '继续调试',
  })) return
  emit('close')
}
function keydown(event) {
  if (event.key !== 'Escape' || event.defaultPrevented || envOpen.value) return
  event.preventDefault()
  close()
}
watch(code, () => { if (runSummary.value) clearDiagnostics() })
onMounted(() => { document.body.classList.add('debug-open'); window.addEventListener('keydown', keydown) })
onBeforeUnmount(() => { document.body.classList.remove('debug-open'); window.removeEventListener('keydown', keydown) })
</script>
<template>
  <Teleport to="body">
    <section class="debug-overlay" role="dialog" aria-modal="true" aria-label="脚本调试">
      <header class="debug-header">
        <div class="debug-title"><span class="debug-mark"><Icon name="Terminal" :size="19" /></span><div><span class="eyebrow">SCRIPT DEBUG</span><h2>{{ path }}</h2><small>{{ dirty ? '左侧内容尚未保存，运行时使用当前编辑内容' : '调试会使用左侧当前内容，不要求先保存' }}</small></div></div>
        <div class="debug-actions"><button class="button compact" @click="envOpen = true"><Icon name="KeyRound" :size="15" />环境变量</button><button class="button compact" :disabled="saving" @click="save"><Icon name="Save" :size="15" />{{ saving ? '保存中' : '保存' }}</button><button v-if="isRunning" class="button danger compact" :disabled="stopping" @click="stopRun"><Icon name="Square" :size="15" />{{ stopping ? '正在停止' : '停止运行' }}</button><button v-else class="button primary compact" :disabled="running" @click="run"><Icon :name="running ? 'LoaderCircle' : 'Play'" :size="15" :class="{ spin: running }" />{{ running ? '正在启动' : '运行当前代码' }}</button><button class="icon-button" aria-label="关闭调试" title="关闭调试" @click="close"><Icon name="X" /></button></div>
      </header>
      <div class="debug-body">
        <div class="debug-code-pane"><CodeEditor ref="editor" v-model="code" :filename="path" @save="save" /></div>
        <div class="debug-log-pane">
          <div class="debug-log-heading"><span><Icon name="FileText" :size="16" />运行日志</span><small>{{ runId ? 'RUN #' + runId : '尚未运行' }}</small></div>
          <p v-if="runError" class="inline-error debug-error">{{ runError }}</p>
          <p v-if="runSummary" class="debug-error-banner"><Icon name="CircleAlert" :size="15" /><span>{{ runSummary }}</span></p>
          <LogPanel v-if="runId" :key="runId" :run-id="runId" :title="'调试 · ' + path" embedded @status="onRunStatus" />
          <div v-else class="empty-state debug-empty"><div class="empty-icon"><Icon name="Terminal" :size="30" /></div><h3>点击「运行当前代码」开始调试</h3><p>左侧修改无需保存；运行时会把当前内容写入临时文件，任务结束后自动清理。</p></div>
        </div>
      </div>
      <EnvironmentDialog v-if="envOpen" @close="envOpen = false" />
    </section>
  </Teleport>
</template>
