<script setup>
import { ref, computed, watch } from 'vue'
import { api, action, toast, usePolling, date, statusLabel } from '../api'
import { confirmDelete, detailList } from '../dialogs'
import Icon from '../components/Icon.vue'
import FormDialog from '../components/FormDialog.vue'
import TaskDetail from '../components/TaskDetail.vue'
import TaskEditorDialog from '../components/TaskEditorDialog.vue'
const props = defineProps({ globalSearch: String })
const tasks = ref([]), search = ref(props.globalSearch || ''), filter = ref('all'), selected = ref([]), page = ref(1), size = ref(20), loading = ref(true), saving = ref(false), modal = ref(null), pending = ref(false), activeTaskId = ref(0), source = ref(null)
const activeTask = computed(() => tasks.value.find(item => item.id === activeTaskId.value) || null)
const fields = [
  { key: 'name', label: '任务名称', required: true, placeholder: '例如：每日数据备份' },
  { key: 'command', label: '执行命令', type: 'textarea', rows: 3, required: true, code: true, placeholder: 'task example.py', hint: '支持 task script.py / node script.js / bash script.sh，以及 Shell 命令。工作目录为 scripts。' },
  { key: 'schedule', label: 'Cron 表达式', placeholder: '0 8 * * *', hint: '支持 5 段、6 段（秒）和 @every 30m。留空表示仅手动执行。' },
  { key: 'remarks', label: '备注', placeholder: '简单描述这个任务' },
  { key: 'labelText', label: '标签', placeholder: '使用逗号分隔，例如：日常,备份' },
  { key: 'timeout', label: '执行超时（秒）', type: 'number', min: 0, max: 604800, hint: '0 表示使用系统默认值。' },
  { key: 'allowConcurrent', label: '允许同一任务重叠执行', type: 'checkbox', hint: '默认关闭：上一次执行尚未结束时，跳过重复提交。' },
]
function createdTime(item) { return new Date(item.createdAt || item.updatedAt || 0).getTime() || item.id }
function taskOrder(left, right) { return Number(!!left.isDisabled) - Number(!!right.isDisabled) || Number(!!right.isPinned) - Number(!!left.isPinned) || createdTime(right) - createdTime(left) || right.id - left.id }
async function load() { try { const result = await api('/crons'); tasks.value = [...result.data].sort(taskOrder); selected.value = selected.value.filter(id => tasks.value.some(item => item.id === id)); if (activeTaskId.value && !tasks.value.some(item => item.id === activeTaskId.value)) activeTaskId.value = 0 } finally { loading.value = false } }
usePolling(load, 3000)
watch(() => props.globalSearch, value => { search.value = value; page.value = 1 })
watch([search, filter, size], () => { page.value = 1 })
const filters = computed(() => [
  { id: 'all', label: '全部', count: tasks.value.length },
  { id: 'running', label: '运行中', count: tasks.value.filter(item => item.status === 0).length },
  { id: 'disabled', label: '已停用', count: tasks.value.filter(item => item.isDisabled).length },
  { id: 'failed', label: '失败', count: tasks.value.filter(item => ['failed', 'timeout', 'interrupted'].includes(item.lastStatus)).length },
])
const filtered = computed(() => tasks.value.filter(item => {
  if (search.value && !JSON.stringify(item).toLowerCase().includes(search.value.toLowerCase())) return false
  if (filter.value === 'running') return item.status === 0
  if (filter.value === 'disabled') return item.isDisabled
  if (filter.value === 'failed') return ['failed', 'timeout', 'interrupted'].includes(item.lastStatus)
  return true
}))
const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / size.value)))
const visible = computed(() => filtered.value.slice((page.value - 1) * size.value, page.value * size.value))
watch(totalPages, count => { if (page.value > count) page.value = count })
function edit(item) { modal.value = item ? { ...item, labelText: (item.labels || []).join(', '), allowConcurrent: !!item.allowConcurrent } : { name: '', command: '', schedule: '0 8 * * *', remarks: '', timeout: 0, allowConcurrent: false, labelText: '' } }
async function save(values) { saving.value = true; try { const body = { ...values, timeout: Number(values.timeout || 0), labels: (values.labelText || '').split(/[,，]/).map(value => value.trim()).filter(Boolean) }; delete body.labelText; await api('/crons', values.id ? 'PUT' : 'POST', body); modal.value = null; toast('任务已保存'); await load() } catch (error) { toast(error.message, 'error') } finally { saving.value = false } }
async function operate(operation, ids) {
  if (pending.value) return
  if (!ids.length) return toast('请先选择任务', 'error')
  if (operation === 'delete') {
    const names = tasks.value.filter(item => ids.includes(item.id)).map(item => item.name)
    if (!await confirmDelete({
      title: ids.length > 1 ? '删除选中的 ' + ids.length + ' 个任务？' : '删除「' + (names[0] || '此任务') + '」？',
      message: '任务会从调度列表中移除，对应的脚本文件不会被删除。',
      details: detailList(names, 4),
      note: '历史运行日志仍会保留。此操作不可撤销。',
      confirmText: '删除任务',
    })) return
  }
  pending.value = true
  await action(async () => { await api('/crons' + (operation === 'delete' ? '' : '/' + operation), operation === 'delete' ? 'DELETE' : 'PUT', ids); await load() }, operation === 'run' ? '任务已加入执行队列' : '操作完成')
  pending.value = false
}
function openDetail(item) { activeTaskId.value = item.id }
function openSource(item) { source.value = item }
function selectAll(event) { selected.value = event.target.checked ? [...new Set([...selected.value, ...visible.value.map(item => item.id)])] : selected.value.filter(id => !visible.value.some(item => item.id === id)) }
</script>
<template>
  <template v-if="!activeTask">
  <div class="section-toolbar"><div class="section-note"><span class="live-dot" />{{ tasks.filter(item => !item.isDisabled).length }} 个任务已启用<span class="separator">/</span>自动调度已就绪</div><button class="button primary" @click="edit()"><Icon name="Plus" />新建任务</button></div>
  <section class="card filter-bar"><div class="search-input"><Icon name="Search" :size="18" /><input v-model="search" placeholder="搜索任务名称、命令或标签…" aria-label="筛选任务" /></div><div class="filter-pills"><button v-for="item in filters" :key="item.id" :class="{ active: filter === item.id }" @click="filter = item.id"><i v-if="item.id !== 'all'" :class="item.id" />{{ item.label }}<span>{{ item.count }}</span></button></div><button class="icon-button" aria-label="刷新任务" @click="action(load)"><Icon name="RefreshCw" /></button></section>
  <div v-if="selected.length" class="bulk-bar"><span>已选择 {{ selected.length }} 项</span><button @click="operate('run', selected)">运行</button><button @click="operate('stop', selected)">停止</button><button @click="operate('pin', selected)">置顶</button><button @click="operate('unpin', selected)">取消置顶</button><button @click="operate('enable', selected)">启用</button><button @click="operate('disable', selected)">停用</button><button class="danger-text" @click="operate('delete', selected)">删除</button><button @click="selected = []">取消选择</button></div>
  <section class="card task-list"><div v-if="visible.length" class="task-list-header"><label><input type="checkbox" :checked="visible.length > 0 && visible.every(item => selected.includes(item.id))" aria-label="选择本页全部任务" @change="selectAll" />选择本页</label><span>调度时间依据服务器时区 · 支持秒级 Cron</span></div>
    <article v-for="item in visible" :key="item.id" class="task-row" :class="{ running: item.status === 0, disabled: item.isDisabled }"><input v-model="selected" type="checkbox" :value="item.id" :aria-label="'选择' + item.name" /><div class="task-state" :class="item.status === 0 ? 'running' : item.lastStatus || 'idle'"><Icon :name="item.status === 0 ? 'LoaderCircle' : ['failed', 'timeout', 'interrupted'].includes(item.lastStatus) ? 'CircleAlert' : item.isDisabled ? 'CirclePause' : 'CirclePlay'" :class="{ spin: item.status === 0 }" :size="26" /></div><div class="task-name"><button class="task-name-button" @click="openDetail(item)"><strong><Icon v-if="item.isPinned" name="Pin" :size="13" />{{ item.name }}</strong></button><span>{{ item.remarks || '点击任务名称查看详情' }}</span><button v-if="item.command" class="task-command" :title="'打开源码：' + item.command" @click.stop="openSource(item)"><code>{{ item.command }}</code></button><div v-if="item.labels?.length" class="labels"><i v-for="label in item.labels" :key="label">{{ label }}</i></div></div><div class="task-schedule"><code>{{ item.schedule || '手动运行' }}</code><small>{{ item.status === 0 ? (item.lastStatus === 'queued' ? '等待执行' : '正在执行 · ' + item.runningCount + ' 实例') : 'Cron 表达式' }}</small></div><div class="task-time"><small>上次运行</small><span>{{ date(item.lastRun) }}</span></div><div class="task-time next"><small>下次运行</small><span>{{ item.isDisabled ? '已停用' : date(item.nextRun) }}</span></div><div class="task-duration"><small>最近耗时</small><span>{{ item.last_execution_time != null ? Number(item.last_execution_time).toFixed(2) + 's' : '—' }}</span></div><button class="toggle" role="switch" :aria-checked="!item.isDisabled" :aria-label="'启用' + item.name" :class="{ on: !item.isDisabled }" :disabled="pending" @click="operate(item.isDisabled ? 'enable' : 'disable', [item.id])"><span /></button><div class="task-actions"><button class="icon-button" :class="{ 'pin-active': item.isPinned }" :title="item.isPinned ? '取消置顶' : '置顶'" :aria-label="(item.isPinned ? '取消置顶' : '置顶') + item.name" :disabled="pending" @click="operate(item.isPinned ? 'unpin' : 'pin', [item.id])"><Icon name="Pin" :size="16" /></button><button class="icon-button" :title="item.status === 0 ? '停止' : '立即运行'" :aria-label="(item.status === 0 ? '停止' : '运行') + item.name" :disabled="pending" @click="operate(item.status === 0 ? 'stop' : 'run', [item.id])"><Icon :name="item.status === 0 ? 'Square' : 'Play'" :size="18" /></button><button class="icon-button" title="查看日志" :aria-label="'查看' + item.name + '的日志'" @click="openDetail(item)"><Icon name="FileText" :size="18" /></button><button class="icon-button" title="编辑任务" :aria-label="'编辑' + item.name" @click="edit(item)"><Icon name="Pencil" :size="18" /></button><button class="icon-button danger" title="删除任务" :aria-label="'删除' + item.name" @click="operate('delete', [item.id])"><Icon name="Trash2" :size="18" /></button></div></article>
    <div v-if="!visible.length" class="empty-state"><div class="empty-icon"><Icon name="Clock3" :size="31" /></div><h3>{{ loading ? '正在加载任务…' : search || filter !== 'all' ? '没有找到匹配的任务' : '您的时间，值得被自动化' }}</h3><p>{{ search || filter !== 'all' ? '试试其他关键词或筛选条件。' : '添加一个脚本命令，剩下的就交给大白菜。' }}</p><button v-if="!search && filter === 'all'" class="button primary" @click="edit()"><Icon name="Plus" />创建第一个任务</button></div>
    <footer class="pagination"><span>共 {{ filtered.length }} 个任务，当前第 {{ page }} 页</span><div><button class="icon-button" aria-label="上一页" :disabled="page <= 1" @click="page--"><Icon name="ChevronLeft" :size="17" /></button><span class="current-page">{{ page }}</span><button class="icon-button" aria-label="下一页" :disabled="page >= totalPages" @click="page++"><Icon name="ChevronRight" :size="17" /></button><select v-model="size" aria-label="每页数量"><option :value="10">10 条 / 页</option><option :value="20">20 条 / 页</option><option :value="50">50 条 / 页</option></select></div></footer>
  </section>
  </template>
  <TaskDetail v-else :key="activeTask.id" :task="activeTask" :pending="pending" @back="activeTaskId = 0" @edit="edit(activeTask)" @source="source = activeTask" @run="operate('run', [activeTask.id])" @stop="operate('stop', [activeTask.id])" />
  <FormDialog v-if="modal" :title="modal.id ? '编辑任务' : '新建任务'" :fields="fields" :initial="modal" :busy="saving" @close="modal = null" @submit="save" />
  <Teleport to="body"><TaskEditorDialog v-if="source" :task="source" @close="source = null" /></Teleport>
</template>
