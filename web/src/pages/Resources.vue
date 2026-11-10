<script setup>
import { ref, computed, watch } from 'vue'
import { api, action, toast, usePolling, date, copy } from '../api'
import { confirmDelete, detailList } from '../dialogs'
import Icon from '../components/Icon.vue'
import FormDialog from '../components/FormDialog.vue'
import LogPanel from '../components/LogPanel.vue'
const props = defineProps({ kind: String })
const items = ref([]), search = ref(''), category = ref(0), envFilter = ref('all'), modal = ref(null), saving = ref(false), run = ref(null)
const selected = ref([]), page = ref(1), size = ref(20), pending = ref(false)
const labels = { envs: '环境变量', subscriptions: '订阅', dependencies: '依赖' }
const dependencyTypes = { 0: 'npm', 1: 'pip', 2: '系统包' }
function createdTime(item) { return new Date(item.createdAt || item.updatedAt || 0).getTime() || item.id }
function sortResources(list) {
  if (props.kind === 'envs') return [...list].sort((left, right) => Number(!!right.isPinned) - Number(!!left.isPinned) || Number(left.position || 0) - Number(right.position || 0) || createdTime(left) - createdTime(right) || left.id - right.id)
  return [...list].sort((left, right) => Number(!!right.isPinned) - Number(!!left.isPinned) || createdTime(right) - createdTime(left) || right.id - left.id)
}
function formatText(value) {
  if (value === undefined || value === null || value === '') return '—'
  if (typeof value === 'object') {
    const text = JSON.stringify(value)
    return text === '{}' ? '—' : text
  }
  return String(value)
}
function formatValue(value) {
  return formatText(value)
}
async function load() { const list = await api('/' + props.kind); items.value = sortResources(list); selected.value = selected.value.filter(id => items.value.some(item => item.id === id)) }
usePolling(load, 4000)
const filtered = computed(() => items.value.filter(item => {
  if (search.value && !JSON.stringify(item).toLowerCase().includes(search.value.toLowerCase())) return false
  if (props.kind !== 'dependencies' || item.type === category.value) {
    if (props.kind === 'envs') {
      if (envFilter.value === 'enabled' && item.status) return false
      if (envFilter.value === 'disabled' && !item.status) return false
      if (envFilter.value === 'pinned' && !item.isPinned) return false
    }
    return true
  }
  return false
}))
const pages = computed(() => Math.max(1, Math.ceil(filtered.value.length / size.value)))
const visible = computed(() => filtered.value.slice((page.value - 1) * size.value, page.value * size.value))
watch([search, category, size, envFilter], () => { page.value = 1 })
watch(pages, value => { if (page.value > value) page.value = value })
function toggleAll(event) { selected.value = event.target.checked ? [...new Set([...selected.value, ...visible.value.map(item => item.id)])] : selected.value.filter(id => !visible.value.some(item => item.id === id)) }
async function bulk(operation) {
  if (!selected.value.length || pending.value) return
  if (operation === 'delete') {
    const names = items.value.filter(item => selected.value.includes(item.id)).map(item => item.name)
    const isDependency = props.kind === 'dependencies'
    if (!await confirmDelete({
      title: '删除选中的 ' + selected.value.length + ' 项' + (isDependency ? '并卸载这些依赖？' : '？'),
      message: isDependency ? '依赖将从环境中卸载，引用它们的脚本可能无法继续运行。' : '所选' + labels[props.kind] + '将被永久删除。',
      details: detailList(names, 4),
      note: isDependency ? '已安装的文件会被移除。此操作不可撤销。' : '此操作不可撤销。',
      confirmText: isDependency ? '卸载' : '删除',
    })) return
  }
  pending.value = true
  await action(async () => { await api('/' + props.kind + (operation === 'delete' ? '' : '/' + operation), operation === 'delete' ? 'DELETE' : 'PUT', selected.value); selected.value = []; await load() }, '批量操作已提交')
  pending.value = false
}
const fields = computed(() => props.kind === 'envs' ? [
  { key: 'name', label: '变量名称', required: true, placeholder: 'MY_TOKEN' },
  { key: 'value', label: '变量值', type: 'textarea', required: true, rows: 4, hint: '仅启用的变量会注入脚本进程；同名变量按排序靠后的值生效。' },
  { key: 'remarks', label: '备注', placeholder: '这个变量用于…' },
] : props.kind === 'dependencies' ? [
  { key: 'name', label: '包名称', required: true, placeholder: category.value === 1 ? 'requests==2.32.5' : category.value === 0 ? 'axios@1.12.2' : 'curl', hint: '每次添加一个包。提交后将异步安装，可查看安装日志。' },
  { key: 'type', label: '依赖类型', type: 'select', options: [{ value: 0, label: 'Node.js (npm)' }, { value: 1, label: 'Python (pip / 独立虚拟环境)' }, { value: 2, label: 'Linux 系统包 (apt / apk)' }] },
  { key: 'remark', label: '备注' },
] : [
  { key: 'name', label: '订阅名称', required: true },
  { key: 'type', label: '订阅类型', type: 'select', options: [{ value: 'git-repo', label: 'Git 仓库' }, { value: 'file', label: '单个脚本文件' }] },
  { key: 'url', label: '订阅 URL', required: true, placeholder: 'https://github.com/owner/repo.git' },
  { key: 'alias', label: '目录别名', required: true, placeholder: 'my-scripts', hint: '字母、数字、连字符或下划线，脚本同步到 scripts/别名/。' },
  { key: 'branch', label: 'Git 分支', placeholder: '留空使用默认分支' },
  { key: 'schedule', label: '同步 Cron', placeholder: '0 */6 * * *', hint: '留空表示仅手动同步。' },
  { key: 'whitelist', label: '文件白名单（正则）', placeholder: '留空同步全部支持的脚本' },
  { key: 'blacklist', label: '文件黑名单（正则）' },
  { key: 'autoAddCron', label: '根据脚本中的 cron: 注释自动添加任务', type: 'checkbox' },
  { key: 'autoDelCron', label: '同步删除源仓库中已移除的脚本与关联任务', type: 'checkbox' },
])
function edit(item) { modal.value = item ? { ...item } : props.kind === 'envs' ? { name: '', value: '', remarks: '' } : props.kind === 'dependencies' ? { name: '', type: category.value, remark: '' } : { name: '', type: 'git-repo', url: '', alias: '', branch: '', schedule: '0 */6 * * *', whitelist: '', blacklist: '', autoAddCron: true, autoDelCron: false, schedule_type: 'crontab' } }
async function save(body) { saving.value = true; try { await api('/' + props.kind, body.id ? 'PUT' : 'POST', !body.id && props.kind !== 'subscriptions' ? [body] : body); modal.value = null; toast(props.kind === 'dependencies' ? '已加入安装队列' : '已保存'); await load() } catch (error) { toast(error.message, 'error') } finally { saving.value = false } }
async function operate(operation, item) {
  if (operation === 'delete') {
    const isDependency = props.kind === 'dependencies'
    if (!await confirmDelete({
      title: (isDependency ? '卸载依赖「' : '删除' + labels[props.kind] + '「') + item.name + '」？',
      message: isDependency ? '依赖将从环境中卸载，引用它的脚本可能无法继续运行。' : '这条' + labels[props.kind] + '将被永久删除。',
      details: isDependency ? [item.name] : [],
      note: isDependency ? '已安装的文件会被移除。此操作不可撤销。' : '此操作不可撤销。',
      confirmText: isDependency ? '卸载' : '删除',
    })) return
  }
  await action(async () => { await api('/' + props.kind + (operation === 'delete' ? '' : '/' + operation), operation === 'delete' ? 'DELETE' : 'PUT', [item.id]); await load() }, ['run', 'reinstall'].includes(operation) ? '已加入后台队列' : '操作完成')
}
async function showLog(item) { await action(async () => { const logs = await api('/logs?kind=' + props.kind + '&taskId=' + item.id); if (!logs.length) return toast('尚无执行记录'); run.value = logs[0] }) }
async function move(item, offset) { const index = items.value.findIndex(value => value.id === item.id); if (index + offset < 0 || index + offset >= items.value.length) return; await action(async () => { await api('/envs/' + item.id + '/move', 'PUT', { fromIndex: index, toIndex: index + offset }); await load() }) }
</script>
<template>
  <div class="section-toolbar"><div v-if="kind === 'dependencies'" class="tabs"><button v-for="(label, index) in ['Node.js', 'Python', '系统依赖']" :key="label" :class="{ active: category === index }" @click="category = index">{{ label }}<span>{{ items.filter(item => item.type === index).length }}</span></button></div><span v-else class="section-note">共 {{ items.length }} 条{{ labels[kind] }}</span><select v-if="kind === 'envs'" v-model="envFilter" class="env-filter" aria-label="筛选环境变量"><option value="all">全部变量</option><option value="enabled">已启用</option><option value="disabled">已停用</option><option value="pinned">已置顶</option></select><button class="button primary" @click="edit()"><Icon name="Plus" />添加{{ labels[kind] }}</button></div>
  <div v-if="kind === 'dependencies'" class="info-banner"><Icon name="Info" /><span>{{ category === 0 ? 'Node 依赖安装到独立目录，默认禁止 npm 生命周期脚本。原生扩展请在可信环境中预装。' : category === 1 ? 'Python 依赖使用面板专属虚拟环境，不修改宿主 Python 环境。' : '系统包安装需要 Linux 宿主具备 apt-get 或 apk 和相应安装权限。' }}</span></div>
  <div v-if="selected.length" class="bulk-bar"><span>已选择 {{ selected.length }} 项</span><template v-if="kind !== 'dependencies'"><button :disabled="pending" @click="bulk('enable')">启用</button><button :disabled="pending" @click="bulk('disable')">停用</button></template><template v-if="kind !== 'envs'"><button :disabled="pending" @click="bulk(kind === 'subscriptions' ? 'run' : 'reinstall')">{{ kind === 'subscriptions' ? '同步' : '重新安装' }}</button><button :disabled="pending" @click="bulk(kind === 'subscriptions' ? 'stop' : 'cancel')">停止</button></template><button class="danger-text" :disabled="pending" @click="bulk('delete')">{{ kind === 'dependencies' ? '卸载' : '删除' }}</button><button @click="selected = []">取消选择</button></div>
  <section class="card"><div class="card-heading"><div class="search-input borderless"><Icon name="Search" /><input v-model="search" :placeholder="'搜索' + labels[kind] + '…'" :aria-label="'搜索' + labels[kind]" /></div><button class="icon-button" aria-label="刷新列表" @click="action(load)"><Icon name="RefreshCw" /></button></div>
    <template v-if="kind === 'envs'"><div class="table-scroll"><table class="env-table"><thead><tr><th class="checkbox-column"><input type="checkbox" aria-label="选择本页记录" :checked="visible.length > 0 && visible.every(item => selected.includes(item.id))" :indeterminate="visible.some(item => selected.includes(item.id)) && !visible.every(item => selected.includes(item.id))" @change="toggleAll" /></th><th>名称</th><th>变量值</th><th>备注</th><th>最后修改</th><th>启用</th><th class="align-right">操作</th></tr></thead><tbody><tr v-for="item in visible" :key="item.id" :class="{ 'selected-row': selected.includes(item.id) }"><td><input v-model="selected" type="checkbox" :value="item.id" :aria-label="'选择记录 ' + item.name" /></td><td><div class="resource-name"><span class="resource-icon"><Icon name="KeyRound" /></span><div class="resource-copy"><div class="resource-title-line"><strong>{{ item.name }}</strong><span class="record-id">#{{ item.id }}</span></div></div></div></td><td><div class="env-value"><code :title="formatValue(item.value)">{{ formatValue(item.value) }}</code><button class="icon-button" aria-label="复制变量值" @click="copy(formatValue(item.value))"><Icon name="Copy" :size="15" /></button></div></td><td class="muted">{{ formatText(item.remarks) }}</td><td class="muted nowrap">{{ date(item.updatedAt || item.createdAt) }}</td><td><button class="toggle" role="switch" :aria-checked="!item.status" :aria-label="'启用' + item.name" :class="{ on: !item.status }" @click="operate(item.status ? 'enable' : 'disable', item)"><span /></button></td><td><div class="actions justify-end"><button class="icon-button" :class="{ 'pin-active': item.isPinned }" :aria-label="(item.isPinned ? '取消置顶' : '置顶') + item.name" :title="item.isPinned ? '取消置顶' : '置顶'" @click="operate(item.isPinned ? 'unpin' : 'pin', item)"><Icon name="Pin" :size="16" /></button><button class="icon-button" aria-label="上移变量" title="上移" @click="move(item, -1)"><Icon name="ArrowUp" :size="16" /></button><button class="icon-button" aria-label="下移变量" title="下移" @click="move(item, 1)"><Icon name="ArrowDown" :size="16" /></button><button class="icon-button" aria-label="编辑记录" title="编辑" @click="edit(item)"><Icon name="Pencil" :size="17" /></button><button class="icon-button danger" aria-label="删除记录" title="删除" @click="operate('delete', item)"><Icon name="Trash2" :size="17" /></button></div></td></tr></tbody></table></div></template>
    <template v-else-if="kind === 'subscriptions'"><div class="table-scroll"><table><thead><tr><th class="checkbox-column"><input type="checkbox" aria-label="选择本页记录" :checked="visible.length > 0 && visible.every(item => selected.includes(item.id))" :indeterminate="visible.some(item => selected.includes(item.id)) && !visible.every(item => selected.includes(item.id))" @change="toggleAll" /></th><th>名称</th><th>同步计划 / 来源</th><th>最近同步</th><th>启用</th><th class="align-right">操作</th></tr></thead><tbody><tr v-for="item in visible" :key="item.id" :class="{ 'selected-row': selected.includes(item.id) }"><td><input v-model="selected" type="checkbox" :value="item.id" :aria-label="'选择记录 ' + item.name" /></td><td><div class="resource-name"><span class="resource-icon"><Icon name="GitBranch" /></span><div class="resource-copy"><div class="resource-title-line"><strong>{{ item.name }}</strong><span class="record-id">#{{ item.id }}</span></div><small class="secondary-line">{{ item.alias }}</small></div></div></td><td><code>{{ item.schedule || '手动同步' }}</code><span class="secondary-line ellipsis resource-url">{{ item.url }}</span></td><td class="muted nowrap">{{ date(item.lastSync) }}</td><td><button class="toggle" role="switch" :aria-checked="!item.isDisabled" :aria-label="'启用' + item.name" :class="{ on: !item.isDisabled }" @click="operate(item.isDisabled ? 'enable' : 'disable', item)"><span /></button></td><td><div class="actions justify-end"><button class="icon-button" :aria-label="'同步订阅'" title="同步" @click="operate('run', item)"><Icon name="RefreshCw" :size="17" /></button><button class="icon-button" aria-label="停止操作" title="停止" @click="operate('stop', item)"><Icon name="Square" :size="16" /></button><button class="icon-button" aria-label="查看执行日志" title="日志" @click="showLog(item)"><Icon name="FileText" :size="17" /></button><button class="icon-button" aria-label="编辑记录" title="编辑" @click="edit(item)"><Icon name="Pencil" :size="17" /></button><button class="icon-button danger" aria-label="删除记录" title="删除" @click="operate('delete', item)"><Icon name="Trash2" :size="17" /></button></div></td></tr></tbody></table></div></template>
    <template v-else><div class="table-scroll"><table><thead><tr><th class="checkbox-column"><input type="checkbox" aria-label="选择本页记录" :checked="visible.length > 0 && visible.every(item => selected.includes(item.id))" :indeterminate="visible.some(item => selected.includes(item.id)) && !visible.every(item => selected.includes(item.id))" @change="toggleAll" /></th><th>名称</th><th>状态</th><th>备注</th><th class="align-right">操作</th></tr></thead><tbody><tr v-for="item in visible" :key="item.id" :class="{ 'selected-row': selected.includes(item.id) }"><td><input v-model="selected" type="checkbox" :value="item.id" :aria-label="'选择记录 ' + item.name" /></td><td><div class="resource-name"><span class="resource-icon"><Icon name="Package" /></span><div class="resource-copy"><div class="resource-title-line"><strong>{{ item.name }}</strong><span class="record-id">#{{ item.id }}</span></div><small class="secondary-line">{{ dependencyTypes[item.type] || '依赖' }}</small></div></div></td><td><span class="badge" :class="item.status === 0 ? 'success' : item.status === 1 ? 'running' : 'failed'">{{ item.status === 0 ? '已安装' : item.status === 1 ? '等待 / 安装中' : '未就绪' }}</span></td><td class="muted">{{ formatText(item.remark) }}</td><td><div class="actions justify-end"><button class="icon-button" aria-label="重新安装" title="重新安装" @click="operate('reinstall', item)"><Icon name="RefreshCw" :size="17" /></button><button class="icon-button" aria-label="停止操作" title="停止" @click="operate('cancel', item)"><Icon name="Square" :size="16" /></button><button class="icon-button" aria-label="查看执行日志" title="日志" @click="showLog(item)"><Icon name="FileText" :size="17" /></button><button class="icon-button" aria-label="编辑记录" title="编辑" @click="edit(item)"><Icon name="Pencil" :size="17" /></button><button class="icon-button danger" aria-label="删除记录" title="删除" @click="operate('delete', item)"><Icon name="Trash2" :size="17" /></button></div></td></tr></tbody></table></div></template>
    <div v-if="!filtered.length" class="empty-state"><div class="empty-icon"><Icon :name="kind === 'envs' ? 'KeyRound' : kind === 'subscriptions' ? 'GitBranch' : 'Package'" :size="31" /></div><h3>尚无{{ labels[kind] }}</h3><p>{{ kind === 'envs' ? '把脚本所需的配置集中在这里，安全而有序。' : kind === 'subscriptions' ? '订阅一个仓库，让脚本更新自动发生。' : '添加依赖，后台会为您完成安装。' }}</p><button class="button" @click="edit()"><Icon name="Plus" :size="17" />添加{{ labels[kind] }}</button></div><footer class="pagination"><span>共 {{ filtered.length }} 条 · 第 {{ page }} / {{ pages }} 页</span><div><select v-model="size" aria-label="每页记录数量"><option :value="20">20 条 / 页</option><option :value="50">50 条 / 页</option><option :value="100">100 条 / 页</option></select><button class="icon-button" aria-label="上一页记录" :disabled="page === 1" @click="page--"><Icon name="ChevronLeft" :size="16" /></button><button class="icon-button" aria-label="下一页记录" :disabled="page === pages" @click="page++"><Icon name="ChevronRight" :size="16" /></button></div></footer></section>
  <FormDialog v-if="modal" :title="(modal.id ? '编辑' : '添加') + labels[kind]" :fields="fields" :initial="modal" :busy="saving" @close="modal = null" @submit="save" />
  <Teleport to="body"><LogPanel v-if="run" :key="run.id" :run-id="run.id" :task-id="run.taskId" :kind="kind" :title="run.name" @close="run = null" /></Teleport>
</template>
