<script setup>
import { ref, computed, onMounted, onBeforeUnmount, defineAsyncComponent } from 'vue'
import { api, toast, action, date } from '../api'
import { confirmDialog, confirmDiscard } from '../dialogs'
import { registerLeaveGuard } from '../guard'
const CodeEditor = defineAsyncComponent(() => import('./CodeEditor.vue'))
import Icon from './Icon.vue'
import LogPanel from './LogPanel.vue'
import ScriptDebugDialog from './ScriptDebugDialog.vue'
const props = defineProps({ path: String, kind: { type: String, default: 'scripts' }, task: Object })
const emit = defineEmits(['dirty', 'saved', 'close', 'expand'])
const content = ref(''), saved = ref(''), loaded = ref(false), loadingError = ref(''), busy = ref(false), revisions = ref(null), preview = ref(null), run = ref(null), runStatus = ref(''), stopping = ref(false), debug = ref(false)
const dirty = computed(() => loaded.value && content.value !== saved.value)
const configHelp = { 'system.json': '并发、超时、日志保留和更新仓库。保存后与系统设置同步生效。', 'notifications.json': '通知渠道与失败通知开关。可在配置文件页的「通知配置」中使用表单编辑。', 'config.sh': '全局 Shell 配置：任务执行前加载，变量自动导出给 Python / JS / Shell。', 'task_before.sh': '任务前置脚本：在同一 Shell 中执行，非零退出码将阻止本次任务。', 'task_after.sh': '任务后置脚本：使用 $CABBAGE_EXIT_CODE 读取任务退出码；不会覆盖原任务结果。' }
async function load() { try { content.value = await api('/' + props.kind + '/detail?path=' + encodeURIComponent(props.path)); saved.value = content.value; loaded.value = true } catch (error) { loadingError.value = error.message } }
function changed(value) { content.value = value; emit('dirty', dirty.value) }
async function save() { if (!loaded.value || busy.value) return false; busy.value = true; const snapshot = content.value; try { await api('/' + props.kind, 'PUT', { filename: props.path, content: snapshot }); saved.value = snapshot; emit('dirty', dirty.value); emit('saved'); toast('文件已保存'); return !dirty.value } catch (error) { toast(error.message, 'error'); return false } finally { busy.value = false } }
async function execute() { if (dirty.value && !await save()) return; await action(async () => { if (props.task) { const result = await api('/crons/run', 'PUT', [props.task.id]); run.value = { id: result[0].runId, taskId: props.task.id }; runStatus.value = 'queued' } else { const result = await api('/scripts/run', 'PUT', { filename: props.path }); run.value = { id: result.runId }; runStatus.value = 'queued' } }, '已加入执行队列') }
function closeRun() { run.value = null; runStatus.value = '' }
async function stopRun() {
  if (stopping.value || !run.value) return
  stopping.value = true
  try {
    if (props.task) await api('/crons/stop', 'PUT', [props.task.id])
    else await api('/scripts/stop', 'PUT', { filename: props.path })
    toast('已请求停止运行')
  } catch (error) { toast(error.message, 'error') } finally { stopping.value = false }
}
async function history() { await action(async () => { revisions.value = (await api('/' + props.kind + '/history?path=' + encodeURIComponent(props.path))).reverse(); preview.value = null }) }
async function viewRevision(item) { await action(async () => { preview.value = await api('/' + props.kind + '/history/detail?id=' + item.id) }) }
async function restore(item) {
  if (!await confirmDialog({
    tone: 'warning',
    icon: 'History',
    eyebrow: '恢复历史版本',
    title: '恢复这个历史版本？',
    message: '当前已保存的内容会先进入历史记录，恢复后编辑器将重新加载。',
    details: [date(item.createdAt)],
    confirmText: '恢复此版本',
    cancelText: '取消',
  })) return
  await action(async () => { await api('/' + props.kind + '/history/restore', 'PUT', { id: item.id }); await load(); revisions.value = null; preview.value = null; emit('dirty', false); emit('saved') }, '历史版本已恢复')
}
async function canLeave() {
  return !dirty.value || await confirmDiscard({
    message: '「' + props.path + '」还有尚未保存的内容，离开后这些修改会丢失。',
  })
}
function beforeUnload(event) { if (dirty.value) { event.preventDefault(); event.returnValue = '' } }
function onDebugClose() {
  debug.value = false
}
function onDebugSaved(value) {
  content.value = value
  saved.value = value
  emit('dirty', dirty.value)
  emit('saved')
}
let unregisterGuard = () => {}
onMounted(() => { load(); window.addEventListener('beforeunload', beforeUnload); unregisterGuard = registerLeaveGuard(canLeave) })
onBeforeUnmount(() => { window.removeEventListener('beforeunload', beforeUnload); unregisterGuard() })
defineExpose({ canLeave, save, dirty })
</script>
<template>
  <div class="file-editor-pane">
    <header class="editor-toolbar"><span><Icon name="FileCode2" :size="17" /><strong :title="path">{{ path }}</strong><i v-if="dirty" class="unsaved-dot" title="未保存" /></span><div class="actions"><button class="icon-button" aria-label="查看历史版本" title="历史版本" @click="history"><Icon name="History" :size="17" /></button><button v-if="task" class="small-button" @click="run = { taskId: task.id }">日志历史</button><button class="icon-button" aria-label="全屏编辑" title="全屏编辑" @click="$emit('expand')"><Icon name="Maximize2" :size="16" /></button><button v-if="kind === 'scripts'" class="button compact" :disabled="!loaded" @click="debug = true"><Icon name="Terminal" :size="15" />调试</button><button v-if="kind === 'scripts'" class="button compact" :disabled="!loaded || busy" @click="execute"><Icon name="Play" :size="15" />运行</button><button class="button primary compact" :disabled="!loaded || busy" @click="save"><Icon name="Save" :size="15" />{{ busy ? '保存中' : '保存' }}</button></div></header>
    <p v-if="kind === 'configs' && configHelp[path]" class="config-help">{{ configHelp[path] }}</p>
    <div v-if="revisions" class="history-list"><div class="card-heading"><h3>保存历史 · {{ revisions.length }} 个版本</h3><button class="icon-button" aria-label="关闭历史版本" @click="revisions = null; preview = null"><Icon name="X" /></button></div><p v-if="!revisions.length" class="muted">修改并保存文件后会自动记录历史版本。</p><div v-for="item in revisions" :key="item.id"><span>{{ date(item.createdAt) }}</span><span class="actions"><button class="small-button" @click="viewRevision(item)">预览</button><button class="small-button" @click="restore(item)">恢复此版本</button></span></div><pre v-if="preview" class="revision-preview">{{ preview.content }}</pre></div>
    <CodeEditor v-if="loaded" :model-value="content" :filename="path" @update:model-value="changed" @save="save" />
    <div v-else class="empty-state"><p :class="loadingError ? 'inline-error' : 'muted'">{{ loadingError || '正在加载文件…' }}</p></div>
    <Teleport to="body"><LogPanel v-if="run" :run-id="run.id" :task-id="run.taskId" :title="task?.name || path" :stoppable="true" @status="runStatus = $event" @stop="stopRun" @close="closeRun" /><ScriptDebugDialog v-if="debug" :path="path" :content="content" @close="onDebugClose" @saved="onDebugSaved" /></Teleport>
  </div>
</template>
