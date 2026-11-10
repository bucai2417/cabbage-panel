<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { api, action } from '../api'
import FileEditor from './FileEditor.vue'
import Icon from './Icon.vue'
import LogPanel from './LogPanel.vue'
const props = defineProps({ task: Object })
const emit = defineEmits(['close'])
const selectedPath = ref(''), files = ref([]), error = ref(''), editor = ref(null), logs = ref(false), loading = ref(true), picker = ref('')
const previousFocus = document.activeElement
async function load() { const results = await Promise.allSettled([api('/crons/' + props.task.id + '/source'), api('/scripts')]); if (results[0].status === 'fulfilled') selectedPath.value = results[0].value.path; else error.value = results[0].reason.message; if (results[1].status === 'fulfilled') files.value = results[1].value; else error.value = results[1].reason.message; loading.value = false }
async function close() { if (!editor.value || await editor.value.canLeave()) emit('close') }
async function openPicked() { if (picker.value && (!editor.value || await editor.value.canLeave())) { selectedPath.value = picker.value; error.value = '' } }
function keyboard(event) {
  if (event.defaultPrevented || document.querySelector('.overlay:not(.task-editor-overlay)')) return
  if (event.key === 'Escape') { event.preventDefault(); close() }
  if (event.key === 'Tab') {
    const elements = [...document.querySelectorAll('.task-editor-modal button:not(:disabled), .task-editor-modal select, .task-editor-modal input, .task-editor-modal [contenteditable="true"]')].filter(element => element.offsetParent !== null)
    const first = elements[0], last = elements.at(-1)
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
  }
}
onMounted(() => { load(); document.body.classList.add('editor-modal-open'); window.addEventListener('keydown', keyboard); document.querySelector('.task-editor-modal button')?.focus() })
onBeforeUnmount(() => { document.body.classList.remove('editor-modal-open'); window.removeEventListener('keydown', keyboard); previousFocus?.focus() })
</script>
<template>
  <div class="overlay task-editor-overlay"><section class="task-editor-modal" role="dialog" aria-modal="true" :aria-label="task.name + ' · 代码编辑'">
    <header class="task-editor-header"><div><h2>{{ task.name }}</h2><code :title="task.command">{{ task.command }}</code></div><div class="actions"><button class="button compact" @click="logs = true"><Icon name="History" :size="15" />日志历史</button><button class="icon-button" aria-label="关闭代码编辑" @click="close"><Icon name="X" /></button></div></header>
    <div class="source-picker"><select v-model="picker" aria-label="选择任务脚本"><option value="">选择其他文件…</option><option v-for="file in files" :key="file.path" :value="file.path">{{ file.path }}</option></select><button class="small-button" :disabled="!picker" @click="openPicked">打开文件</button><span class="muted">{{ task.schedule || '手动运行' }} · 保存脚本后，下次执行生效</span></div>
    <FileEditor v-if="selectedPath" :key="selectedPath" ref="editor" :path="selectedPath" :task="task" />
    <div v-else class="empty-state"><Icon name="FileCode2" :size="38" /><h3>{{ loading ? '正在定位任务脚本…' : '请选择要编辑的脚本' }}</h3><p>{{ error }}</p></div>
  </section><LogPanel v-if="logs" :task-id="task.id" :title="task.name" @close="logs = false" /></div>
</template>
