<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { api, action, toast, bytes } from '../api'
import { confirmDelete, detailList } from '../dialogs'
import Icon from '../components/Icon.vue'
import FormDialog from '../components/FormDialog.vue'
import FileEditor from '../components/FileEditor.vue'
import NotificationSettings from '../components/NotificationSettings.vue'
const props = defineProps({ initialMode: { type: String, default: 'scripts' } })
const mode = ref(props.initialMode), files = ref([]), active = ref(''), expanded = ref(new Set()), selected = ref([]), search = ref(''), modal = ref(null), saving = ref(false), editor = ref(null), menu = ref(null), fullScreen = ref(false), configTab = ref('files'), notificationForm = ref(null), operationErrors = ref([]), createMenu = ref(false)
const directories = computed(() => files.value.filter(file => !file.isLeaf).sort((left, right) => left.path.localeCompare(right.path, 'zh-CN')))
const visible = computed(() => {
  const children = new Map(), included = new Set(), query = search.value.trim().toLowerCase()
  if (query) for (const entry of files.value) if (entry.path.toLowerCase().includes(query)) { let name = entry.path; while (name) { included.add(name); name = parent(name) } }
  for (const entry of files.value) { const directory = parent(entry.path); if (!children.has(directory)) children.set(directory, []); children.get(directory).push(entry) }
  const rows = []
  function walk(directory, depth) { const entries = (children.get(directory) || []).sort((left, right) => Number(left.isLeaf) - Number(right.isLeaf) || left.name.localeCompare(right.name, 'zh-CN')); for (const entry of entries) { if (query && !included.has(entry.path)) continue; rows.push({ ...entry, depth }); if (!entry.isLeaf && (query || expanded.value.has(entry.path))) walk(entry.path, depth + 1) } }
  walk('', 0); return rows
})
const selectable = computed(() => visible.value.filter(file => !file.builtin))
const currentDirectory = computed(() => { const item = files.value.find(file => file.path === active.value); return item && !item.isLeaf ? item.path : parent(active.value) })
function parent(name) { return name.includes('/') ? name.slice(0, name.lastIndexOf('/')) : '' }
function expandParents(name) { let directory = parent(name); const next = new Set(expanded.value); while (directory) { next.add(directory); directory = parent(directory) } expanded.value = next }
async function load() { files.value = await api('/' + mode.value + '/tree'); selected.value = selected.value.filter(name => files.value.some(file => file.path === name)); if (active.value && !files.value.some(file => file.path === active.value)) active.value = '' }
async function canLeave() { return (!editor.value || await editor.value.canLeave()) && (!notificationForm.value || await notificationForm.value.canLeave()) }
async function open(file) { if (!file) return; if (!file.isLeaf) { const next = new Set(expanded.value); if (next.has(file.path)) next.delete(file.path); else next.add(file.path); expanded.value = next; return } if (active.value === file.path || !await canLeave()) return; active.value = file.path; expandParents(file.path) }
async function switchMode(value) { if (value === mode.value || !await canLeave()) return; mode.value = value; active.value = ''; selected.value = []; search.value = ''; expanded.value = new Set(); configTab.value = 'files'; await action(load) }
async function switchConfig(value) { if (configTab.value === value || !await canLeave()) return; configTab.value = value }
function create(type, directory = currentDirectory.value) { modal.value = { type, initial: { filename: directory ? directory + '/' : '' }, paths: [] }; menu.value = null }
async function operate(operation, paths) {
  menu.value = null
  if (!paths.length) return
  if (operation !== 'copy' && !await canLeave()) return
  if (operation === 'delete') {
    const folders = paths.filter(path => { const file = files.value.find(item => item.path === path); return file && !file.isLeaf }).length
    if (!await confirmDelete({
      title: paths.length > 1 ? '删除选中的 ' + paths.length + ' 项？' : '删除「' + paths[0] + '」？',
      message: folders ? '文件夹及其中的全部内容都会被删除。' : '所选脚本或配置会被永久删除。',
      details: detailList(paths, 4),
      note: '已有任务记录会保留，但引用这些脚本的任务可能无法继续运行。此操作不可撤销。',
      confirmText: '删除',
    })) return
    submitOperation(operation, paths)
    return
  }
  modal.value = { type: operation, paths, initial: { destination: operation === 'rename' ? paths[0] : '' } }
}
async function submitOperation(operation, paths, destination = '') { saving.value = true; try { const result = await api('/' + mode.value + '/operations', 'POST', { operation, paths, destination }); operationErrors.value = result.filter(item => !item.success); const successes = result.filter(item => item.success); for (const item of successes) { if (active.value === item.path || active.value.startsWith(item.path + '/')) { if (operation === 'delete') active.value = ''; if (['move', 'rename'].includes(operation)) active.value = item.destination + active.value.slice(item.path.length) } } if (active.value) expandParents(active.value); await load(); selected.value = operationErrors.value.map(item => item.path); modal.value = null; const updated = successes.reduce((sum, item) => sum + item.updatedTasks, 0); toast('完成 ' + successes.length + ' 项' + (updated ? '，同步更新 ' + updated + ' 个任务路径' : ''), operationErrors.value.length ? 'error' : 'success') } catch (error) { toast(error.message, 'error') } finally { saving.value = false } }
async function submit(values) {
  if (!['file', 'folder', 'upload', 'remote'].includes(modal.value.type)) return submitOperation(modal.value.type, modal.value.paths, values.destination)
  if (!await canLeave()) return
  saving.value = true
  try {
    const type = modal.value.type
    if (type === 'upload') {
      if (!values.file) throw new Error('请选择要上传的文件')
      const form = new FormData()
      form.append('file', values.file)
      if (values.directory) form.append('path', values.directory)
      const result = await api('/' + mode.value + '/upload', 'POST', form)
      modal.value = null; await load(); expandParents(result.path); active.value = result.path
      toast('已上传 ' + result.path)
      return
    }
    if (type === 'remote') {
      if (!values.url) throw new Error('请填写外链地址')
      if (!values.filename) throw new Error('请填写保存的文件名')
      await api('/' + mode.value + '/download', 'POST', { url: values.url, filename: values.filename, path: values.directory || '' })
      const saved = (values.directory ? values.directory + '/' : '') + values.filename
      modal.value = null; await load(); expandParents(saved); active.value = saved
      toast('已从外链创建 ' + saved)
      return
    }
    await api('/' + mode.value + (type === 'folder' ? '/folder' : ''), 'POST', type === 'folder' ? { path: values.filename } : { filename: values.filename, content: values.content || '' })
    modal.value = null
    await load()
    expandParents(values.filename)
    if (type === 'folder') expanded.value = new Set([...expanded.value, values.filename])
    else active.value = values.filename
    toast(type === 'folder' ? '文件夹已创建' : '文件已创建')
  } catch (error) { toast(error.message, 'error') } finally { saving.value = false }
}
const fields = computed(() => {
  if (!modal.value) return []
  const type = modal.value.type
  if (type === 'upload') return [
    { key: 'directory', label: '目标文件夹', type: 'select', options: [{ value: '', label: '/（根目录）' }, ...directories.value.map(file => ({ value: file.path, label: file.path }))], hint: '上传后会放在该文件夹下。' },
    { key: 'file', label: '选择本地文件', type: 'file', required: true, hint: '支持任意文本或脚本文件，最大 32MB；同名文件不会被覆盖。' },
  ]
  if (type === 'remote') return [
    { key: 'directory', label: '目标文件夹', type: 'select', options: [{ value: '', label: '/（根目录）' }, ...directories.value.map(file => ({ value: file.path, label: file.path }))] },
    { key: 'filename', label: '保存文件名', required: true, placeholder: 'example.py', hint: '可带子目录，例如 tools/example.py。' },
    { key: 'url', label: '外链地址', required: true, placeholder: 'https://example.com/script.py', hint: '仅支持 HTTP(S)，最大 4MB，内容需为 UTF-8 文本。' },
  ]
  if (['file', 'folder'].includes(type)) return [{ key: 'filename', label: type === 'folder' ? '文件夹路径' : '文件路径', required: true, placeholder: type === 'folder' ? '工具/每日任务' : '工具/example.py', hint: '路径使用 / 分隔，支持中文名称。' }]
  if (type === 'rename') return [{ key: 'destination', label: '新路径', required: true }]
  return [{ key: 'destination', label: '目标文件夹', type: 'select', options: [{ value: '', label: '/（根目录）' }, ...directories.value.map(file => ({ value: file.path, label: file.path }))], hint: '保留原文件名；已存在的目标不会覆盖。' }]
})
const titles = { file: '新建文件', folder: '新建文件夹', upload: '从本地上传', remote: '从外链创建', move: '移动所选文件', copy: '复制所选文件', rename: '重命名' }
function showMenu(file, event) { const bounds = event.currentTarget.getBoundingClientRect(); menu.value = { file, left: Math.max(8, Math.min(bounds.right - 190, window.innerWidth - 200)), top: Math.max(8, Math.min(bounds.bottom + 4, window.innerHeight - 320)) } }
function closeMenu(event) { if (!event.target.closest('.file-context-menu') && !event.target.closest('.file-menu-trigger') && !event.target.closest('.create-menu-wrap')) { menu.value = null; createMenu.value = false } }
function keyboard(event) { if (event.key === 'Escape' && !event.defaultPrevented) { menu.value = null; if (!modal.value) fullScreen.value = false } }
function selectAll(event) { selected.value = event.target.checked ? selectable.value.map(file => file.path) : [] }
function download(file) { action(async () => { const content = await api('/' + mode.value + '/detail?path=' + encodeURIComponent(file.path)); const address = URL.createObjectURL(new Blob([content], { type: 'text/plain;charset=utf-8' })); const link = document.createElement('a'); link.href = address; link.download = file.name; link.click(); setTimeout(() => URL.revokeObjectURL(address), 1000) }); menu.value = null }
function toggleCreateMenu() { createMenu.value = !createMenu.value }
// 批量下载：选中文件打包成 ZIP；未选中时下载当前打开的文件。
async function downloadSelected() {
  const paths = selected.value.length ? selected.value.filter(path => files.value.find(file => file.path === path && file.isLeaf)) : (active.value ? [active.value] : [])
  if (!paths.length) return toast('请选择要下载的文件', 'error')
  try {
    const response = await fetch('/api/' + mode.value + '/archive', {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(paths),
    })
    if (!response.ok) {
      let message = '打包下载失败'
      try { message = (await response.json()).message || message } catch { /* 保留默认提示 */ }
      throw new Error(message)
    }
    const blob = await response.blob()
    const address = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = address
    link.download = mode.value + '-' + new Date().toISOString().slice(0, 19).replace(/[:T]/g, '') + '.zip'
    link.click()
    setTimeout(() => URL.revokeObjectURL(address), 1000)
    toast('已打包 ' + paths.length + ' 个文件')
  } catch (error) { toast(error.message, 'error') }
}
// 拖拽上传：拖入文件树或编辑区即上传，拖到文件夹行则放入该文件夹。
const dropActive = ref(false), dropFolder = ref('')
let dragDepth = 0
function isFileDrag(event) { return Array.from(event.dataTransfer?.types || []).includes('Files') }
function folderAt(event) {
  const row = event.target instanceof Element ? event.target.closest('.tree-row') : null
  if (!row || row.dataset.leaf === 'true') return ''
  return row.dataset.path || ''
}
function onDragEnter(event) {
  if (!isFileDrag(event)) return
  event.preventDefault()
  dragDepth++
  dropActive.value = true
}
function onDragOver(event) {
  if (!isFileDrag(event)) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  dropFolder.value = folderAt(event)
}
function onDragLeave(event) {
  if (!isFileDrag(event)) return
  dragDepth = Math.max(0, dragDepth - 1)
  if (dragDepth === 0) { dropActive.value = false; dropFolder.value = '' }
}
async function uploadFiles(list, directory) {
  if (!list.length) return
  if (!await canLeave()) return
  saving.value = true
  let done = 0
  const errors = []
  for (const file of list) {
    try {
      const form = new FormData()
      form.append('file', file)
      if (directory) form.append('path', directory)
      const result = await api('/' + mode.value + '/upload', 'POST', form)
      done++
      expandParents(result.path)
      active.value = result.path
    } catch (error) { errors.push(file.name + '：' + error.message) }
  }
  await load()
  saving.value = false
  if (errors.length) toast(errors[0] + (errors.length > 1 ? ' 等 ' + errors.length + ' 个失败' : ''), 'error')
  else toast('已上传 ' + done + ' 个文件到 ' + (directory || '根目录'))
}
async function onDrop(event) {
  if (!isFileDrag(event)) return
  event.preventDefault()
  const directory = dropFolder.value
  dragDepth = 0
  dropActive.value = false
  dropFolder.value = ''
  await uploadFiles(Array.from(event.dataTransfer?.files || []), directory)
}
onMounted(() => { action(load); document.addEventListener('pointerdown', closeMenu); window.addEventListener('keydown', keyboard) })
onBeforeUnmount(() => { document.removeEventListener('pointerdown', closeMenu); window.removeEventListener('keydown', keyboard) })
</script>
<template>
  <div class="section-toolbar"><div class="tabs"><button :class="{ active: mode === 'scripts' }" @click="switchMode('scripts')">脚本文件</button><button :class="{ active: mode === 'configs' }" @click="switchMode('configs')">配置文件</button></div><div v-if="configTab === 'files'" class="actions"><button class="button compact" @click="create('folder')"><Icon name="FolderPlus" :size="16" />新建文件夹</button><div class="create-menu-wrap"><button class="button primary compact" @click="toggleCreateMenu"><Icon name="Plus" :size="16" />新建文件</button><div v-if="createMenu" class="file-context-menu create-menu" role="menu"><button role="menuitem" @click="create('file'); createMenu = false"><Icon name="FilePlus" :size="15" />创建空文件</button><button role="menuitem" @click="create('upload'); createMenu = false"><Icon name="Upload" :size="15" />从本地上传</button><button role="menuitem" @click="create('remote'); createMenu = false"><Icon name="Link" :size="15" />从外链创建</button></div></div></div></div>
  <div v-if="mode === 'configs'" class="config-tabs tabs"><button :class="{ active: configTab === 'files' }" @click="switchConfig('files')">系统配置与执行钩子</button><button :class="{ active: configTab === 'notifications' }" @click="switchConfig('notifications')"><Icon name="Bell" :size="15" />通知配置</button></div>
  <NotificationSettings v-if="mode === 'configs' && configTab === 'notifications'" ref="notificationForm" />
  <template v-else><div v-if="selected.length" class="bulk-bar"><span>已选择 {{ selected.length }} 项</span><button :disabled="saving" @click="downloadSelected"><Icon name="Download" :size="15" />下载</button><button :disabled="saving" @click="operate('move', selected)">移动</button><button :disabled="saving" @click="operate('copy', selected)">复制</button><button class="danger-text" :disabled="saving" @click="operate('delete', selected)">删除</button><button @click="selected = []">取消选择</button></div><div v-if="operationErrors.length" class="operation-errors" role="alert"><div v-for="item in operationErrors" :key="item.path">{{ item.path }}：{{ item.error }}</div><button class="small-button" @click="operationErrors = []">关闭</button></div>
  <section class="card editor-layout" :class="{ 'workspace-fullscreen': fullScreen, 'drop-active': dropActive }" @dragenter="onDragEnter" @dragover="onDragOver" @dragleave="onDragLeave" @drop="onDrop"><div v-if="dropActive" class="drop-overlay"><div class="drop-card"><Icon name="Upload" :size="30" /><strong>释放以上传文件</strong><span>目标：{{ dropFolder || currentDirectory || '根目录' }} · 支持多文件</span></div></div><aside class="file-browser"><div class="search-input"><Icon name="Search" :size="16" /><input v-model="search" aria-label="搜索文件" placeholder="搜索文件与文件夹…" /></div><div class="file-browser-heading"><label class="inline-check"><input type="checkbox" aria-label="选择可见文件" :checked="selectable.length > 0 && selectable.every(file => selected.includes(file.path))" :indeterminate="selected.length > 0 && !selectable.every(file => selected.includes(file.path))" @change="selectAll" />{{ files.filter(file => file.isLeaf).length }} 文件</label><div class="actions"><button class="icon-button" aria-label="展开全部文件夹" title="展开全部" @click="expanded = new Set(directories.map(file => file.path))"><Icon name="FolderOpen" :size="15" /></button><button class="icon-button" aria-label="折叠全部文件夹" title="折叠全部" @click="expanded = new Set()"><Icon name="Folder" :size="15" /></button><button class="icon-button" aria-label="刷新文件" @click="action(load)"><Icon name="RefreshCw" :size="15" /></button></div></div>
  <div class="file-tree" role="tree" aria-label="文件夹树"><div v-for="file in visible" :key="file.path" class="tree-row" :class="{ active: active === file.path, selected: selected.includes(file.path), 'drop-target': dropActive && dropFolder === file.path }" :style="{ paddingLeft: 8 + file.depth * 15 + 'px' }" :data-path="file.path" :data-leaf="file.isLeaf ? 'true' : 'false'" role="treeitem" :aria-level="file.depth + 1" :aria-expanded="file.isLeaf ? undefined : expanded.has(file.path) || !!search" :aria-selected="active === file.path"><input v-if="!file.builtin" v-model="selected" type="checkbox" :value="file.path" :aria-label="'选择文件 ' + file.path" /><span v-else class="builtin-file-marker" title="内置配置"><Icon name="LockKeyhole" :size="13" /></span><button class="tree-file-button" :title="file.path + (file.isLeaf ? ' · ' + bytes(file.size) : '')" @click="open(file)"><Icon :name="file.isLeaf ? 'FileCode2' : expanded.has(file.path) || search ? 'FolderOpen' : 'Folder'" :size="16" /><span>{{ file.name }}</span></button><button class="icon-button file-menu-trigger" :aria-label="'文件菜单 ' + file.path" aria-haspopup="menu" @click.stop="showMenu(file, $event)"><Icon name="MoreHorizontal" :size="16" /></button></div><p v-if="!visible.length" class="file-empty">{{ search ? '没有匹配的文件' : '暂无文件，可以创建文件夹或脚本。' }}</p></div></aside>
  <div class="editor-main"><div v-if="fullScreen" class="fullscreen-strip"><span>全屏工作区</span><button class="small-button" @click="fullScreen = false">退出全屏 · Esc</button></div><FileEditor v-if="active" :key="mode + active" ref="editor" :path="active" :kind="mode" @saved="load" @expand="fullScreen = !fullScreen" /><div v-else class="empty-state editor-empty"><div class="empty-icon"><Icon name="Code2" :size="34" /></div><h3>{{ mode === 'configs' ? '配置工作环境' : '选择一个脚本，开始编辑' }}</h3><p>{{ mode === 'configs' ? 'system.json 管理系统参数，notifications.json 管理通知，Shell 配置与前后置钩子在任务执行时生效。' : '支持文件夹树、多选、移动复制与创建定时任务。' }}</p><button class="button" @click="mode === 'configs' ? open(files.find(file => file.path === 'system.json')) : create('file')">{{ mode === 'configs' ? '打开系统配置' : '新建脚本' }}</button></div></div></section></template>
  <FormDialog v-if="modal" :title="titles[modal.type]" :fields="fields" :initial="modal.initial" :busy="saving" @close="modal = null" @submit="submit" />
  <Teleport to="body"><div v-if="menu" class="file-context-menu" role="menu" :style="{ left: menu.left + 'px', top: menu.top + 'px' }"><button v-if="menu.file.isLeaf" role="menuitem" @click="open(menu.file); menu = null"><Icon name="Pencil" :size="15" />打开编辑</button><template v-else><button role="menuitem" @click="create('file', menu.file.path)"><Icon name="Plus" :size="15" />新建文件</button><button role="menuitem" @click="create('folder', menu.file.path)"><Icon name="FolderPlus" :size="15" />新建子文件夹</button></template><button v-if="menu.file.isLeaf" role="menuitem" @click="download(menu.file)"><Icon name="Download" :size="15" />下载文件</button><template v-if="!menu.file.builtin"><button role="menuitem" @click="operate('rename', [menu.file.path])"><Icon name="Pencil" :size="15" />重命名</button><button role="menuitem" @click="operate('move', [menu.file.path])"><Icon name="Move" :size="15" />移动</button><button role="menuitem" @click="operate('copy', [menu.file.path])"><Icon name="Copy" :size="15" />复制</button><button role="menuitem" class="danger-text" @click="operate('delete', [menu.file.path])"><Icon name="Trash2" :size="15" />删除</button></template></div></Teleport>
</template>
