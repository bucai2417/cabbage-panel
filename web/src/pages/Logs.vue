<script setup>
import { ref, computed, watch } from 'vue'
import { api, action, usePolling, date, statusLabel } from '../api'
import { confirmDelete, detailList } from '../dialogs'
import Icon from '../components/Icon.vue'
import LogPanel from '../components/LogPanel.vue'

const records = ref([]), search = ref(''), kindFilter = ref(''), page = ref(1), size = ref(12), busy = ref(false)
const activeGroupKey = ref(''), activeRun = ref(null), checked = ref([])
const runSearch = ref(''), runStatus = ref(''), runPage = ref(1), runSize = ref(20)
const kindLabels = { crons: '定时任务', scripts: '脚本运行', subscriptions: '订阅同步', dependencies: '依赖安装', updates: '版本更新' }
const kindIcons = { crons: 'Clock3', scripts: 'SquareTerminal', subscriptions: 'GitBranch', dependencies: 'Package', updates: 'Download' }
const runTime = run => new Date(run.started || run.created || 0).getTime()
const running = run => ['running', 'queued'].includes(run.status)

const groups = computed(() => {
  const map = new Map()
  for (const run of records.value) {
    const key = run.kind + ':' + (run.taskId || 'name:' + run.name)
    let group = map.get(key)
    if (!group) {
      group = { key, name: run.name, kind: run.kind, taskId: run.taskId || 0, runs: [], latest: run, success: 0, failed: 0, duration: 0 }
      map.set(key, group)
    }
    group.runs.push(run)
    if (run.status === 'success') group.success++
    else if (!running(run) && run.status !== 'cancelled') group.failed++
    group.duration += Number(run.duration || 0)
    if (runTime(run) > runTime(group.latest)) group.latest = run
  }
  const result = [...map.values()]
  for (const group of result) {
    group.runs.sort((left, right) => right.id - left.id)
    group.latest = group.runs[0]
  }
  return result.sort((left, right) => runTime(right.latest) - runTime(left.latest))
})
const filteredGroups = computed(() => groups.value.filter(group => {
  if (kindFilter.value && group.kind !== kindFilter.value) return false
  const query = search.value.trim().toLowerCase()
  return !query || group.name.toLowerCase().includes(query) || group.runs.some(run => (run.command || '').toLowerCase().includes(query))
}))
const pages = computed(() => Math.max(1, Math.ceil(filteredGroups.value.length / size.value)))
const visibleGroups = computed(() => filteredGroups.value.slice((page.value - 1) * size.value, page.value * size.value))
const activeGroup = computed(() => groups.value.find(group => group.key === activeGroupKey.value) || null)
const filteredRuns = computed(() => {
  if (!activeGroup.value) return []
  const query = runSearch.value.trim().toLowerCase()
  return activeGroup.value.runs.filter(run => {
    if (runStatus.value && run.status !== runStatus.value) return false
    return !query || (run.name + ' ' + (run.command || '') + ' #' + run.id).toLowerCase().includes(query)
  })
})
const runPages = computed(() => Math.max(1, Math.ceil(filteredRuns.value.length / runSize.value)))
const visibleRuns = computed(() => filteredRuns.value.slice((runPage.value - 1) * runSize.value, runPage.value * runSize.value))
const deletableRuns = computed(() => visibleRuns.value.filter(run => !running(run)))

watch([search, kindFilter, size], () => { page.value = 1 })
watch(pages, value => { if (page.value > value) page.value = value })
watch([runSearch, runStatus, runSize], () => { runPage.value = 1 })
watch(runPages, value => { if (runPage.value > value) runPage.value = value })
watch(activeGroupKey, key => {
  checked.value = []; runSearch.value = ''; runStatus.value = ''; runPage.value = 1
  activeRun.value = groups.value.find(group => group.key === key)?.runs[0] || null
})

function successRate(group) {
  const finished = group.success + group.failed
  return finished ? Math.round(group.success / finished * 100) : null
}
function openGroup(group) { activeGroupKey.value = group.key }
function closeGroup() { activeGroupKey.value = ''; activeRun.value = null; checked.value = [] }
function toggleAll(event) {
  checked.value = event.target.checked ? [...new Set([...checked.value, ...deletableRuns.value.map(run => run.id)])] : checked.value.filter(id => !deletableRuns.value.some(run => run.id === id))
}
async function load() {
  records.value = await api('/logs?limit=1000')
  checked.value = checked.value.filter(id => { const run = records.value.find(item => item.id === id); return run && !running(run) })
  if (activeRun.value) activeRun.value = records.value.find(run => run.id === activeRun.value.id) || null
  if (activeGroupKey.value && !activeRun.value) activeRun.value = groups.value.find(group => group.key === activeGroupKey.value)?.runs[0] || null
}
usePolling(load, 5000)
async function remove(ids) {
  if (busy.value || !ids.length) return
  if (!await confirmDelete({
    title: '删除 ' + ids.length + ' 条运行记录？',
    message: '对应的日志文件会从磁盘移除，运行记录不再出现在日志页。',
    details: detailList(ids.map(id => '运行 #' + id), 4),
    note: '运行中或排队中的记录不会被删除。此操作不可撤销。',
    confirmText: '删除日志',
  })) return
  busy.value = true
  await action(async () => { await api('/logs', 'DELETE', ids); checked.value = []; await load() }, '日志已删除')
  busy.value = false
}
</script>
<template>
  <template v-if="!activeGroup">
    <div class="section-toolbar"><div class="section-note"><span class="live-dot" />按任务名称归组 · {{ filteredGroups.length }} 个任务 / {{ records.length }} 条记录</div><button class="button compact" @click="action(load)"><Icon name="RefreshCw" :size="16" />刷新</button></div>
    <section class="card filter-bar"><div class="search-input"><Icon name="Search" /><input v-model="search" placeholder="搜索任务名称或命令…" aria-label="搜索运行日志" /></div><select v-model="kindFilter" aria-label="筛选任务类型"><option value="">全部类型</option><option v-for="(label, value) in kindLabels" :key="value" :value="value">{{ label }}</option></select><span class="muted grow">点击任务名称查看该任务的全部运行记录</span></section>
    <section class="card log-group-card">
      <div v-if="visibleGroups.length" class="log-group-header"><span>任务名称</span><span>最近状态</span><span>历史记录</span><span>最近运行</span><span /></div>
      <button v-for="group in visibleGroups" :key="group.key" class="log-group-row" @click="openGroup(group)">
        <span class="log-group-icon"><Icon :name="kindIcons[group.kind] || 'FileText'" :size="19" /></span>
        <span class="log-group-name"><strong>{{ group.name }}</strong><small>{{ kindLabels[group.kind] || group.kind }}<template v-if="group.taskId"> · #{{ group.taskId }}</template></small></span>
        <span class="log-group-status"><span class="badge" :class="group.latest.status"><i />{{ statusLabel[group.latest.status] }}</span></span>
        <span class="log-group-history"><strong>{{ group.runs.length }}</strong><small>次运行<template v-if="successRate(group) !== null"> · 成功率 {{ successRate(group) }}%</template></small></span>
        <span class="log-group-latest"><small>最近运行</small><span>{{ date(group.latest.started || group.latest.created) }}</span></span>
        <Icon name="ChevronRight" :size="18" class="log-group-arrow" />
      </button>
      <div v-if="!visibleGroups.length" class="empty-state"><div class="empty-icon"><Icon name="FileClock" :size="31" /></div><h3>{{ records.length ? '没有匹配的任务' : '暂无运行记录' }}</h3><p>{{ records.length ? '试试其他关键词或任务类型。' : '执行任务后，运行记录会按任务自动归组。' }}</p></div>
      <footer class="pagination"><span>共 {{ filteredGroups.length }} 个任务 · 第 {{ page }} / {{ pages }} 页</span><div><select v-model="size" aria-label="每页任务数量"><option :value="12">12 个 / 页</option><option :value="24">24 个 / 页</option><option :value="48">48 个 / 页</option></select><button class="icon-button" aria-label="上一页任务" :disabled="page === 1" @click="page--"><Icon name="ChevronLeft" :size="16" /></button><button class="icon-button" aria-label="下一页任务" :disabled="page === pages" @click="page++"><Icon name="ChevronRight" :size="16" /></button></div></footer>
    </section>
  </template>
  <template v-else>
    <div class="log-detail-head"><button class="back-link" @click="closeGroup"><Icon name="ChevronLeft" :size="16" />返回任务分组</button><div class="log-detail-title"><span class="log-group-icon"><Icon :name="kindIcons[activeGroup.kind] || 'FileText'" :size="20" /></span><div><h2>{{ activeGroup.name }}</h2><small>{{ kindLabels[activeGroup.kind] || activeGroup.kind }} · 共 {{ activeGroup.runs.length }} 条运行记录</small></div></div><span class="badge" :class="activeGroup.latest.status"><i />{{ statusLabel[activeGroup.latest.status] }}</span></div>
    <div class="log-detail-grid">
      <section class="card log-run-list">
        <div class="card-heading log-run-toolbar"><div class="search-input borderless"><Icon name="Search" :size="16" /><input v-model="runSearch" placeholder="筛选这个任务的运行记录…" aria-label="筛选任务运行记录" /></div><select v-model="runStatus" aria-label="筛选运行状态"><option value="">全部状态</option><option v-for="(label, value) in statusLabel" :key="value" :value="value">{{ label }}</option></select></div>
        <div v-if="checked.length" class="bulk-bar"><span>已选择 {{ checked.length }} 条</span><button class="danger-text" :disabled="busy" @click="remove(checked)">删除所选日志</button><button @click="checked = []">取消选择</button></div>
        <div v-if="visibleRuns.length" class="log-run-head"><label class="inline-check"><input type="checkbox" aria-label="选择本页日志" :checked="deletableRuns.length > 0 && deletableRuns.every(run => checked.includes(run.id))" :indeterminate="deletableRuns.some(run => checked.includes(run.id)) && !deletableRuns.every(run => checked.includes(run.id))" @change="toggleAll" />选择本页</label><span>{{ filteredRuns.length }} 条记录</span></div>
        <div v-for="run in visibleRuns" :key="run.id" class="log-run-row" :class="{ active: activeRun?.id === run.id }">
          <input v-model="checked" type="checkbox" :value="run.id" :disabled="running(run)" :aria-label="'选择日志 ' + run.id" />
          <button class="log-run-main" @click="activeRun = run">
            <span class="badge" :class="run.status"><i />{{ statusLabel[run.status] }}</span>
            <code class="run-id">#{{ run.id }}</code>
            <span class="log-run-time">{{ date(run.started || run.created) }}</span>
            <span class="log-run-meta">{{ Number(run.duration || 0).toFixed(2) }}s · 退出码 {{ run.exitCode ?? '—' }}</span>
          </button>
          <button class="icon-button danger" aria-label="删除日志" :disabled="busy || running(run)" @click="remove([run.id])"><Icon name="Trash2" :size="16" /></button>
        </div>
        <div v-if="!visibleRuns.length" class="empty-state compact-empty"><Icon name="FileClock" :size="28" /><h3>没有匹配的运行记录</h3><p>调整筛选条件后再试。</p></div>
        <footer class="pagination"><span>共 {{ filteredRuns.length }} 条 · 第 {{ runPage }} / {{ runPages }} 页</span><div><select v-model="runSize" aria-label="每页运行记录数量"><option :value="20">20 条 / 页</option><option :value="50">50 条 / 页</option><option :value="100">100 条 / 页</option></select><button class="icon-button" aria-label="上一页运行记录" :disabled="runPage === 1" @click="runPage--"><Icon name="ChevronLeft" :size="16" /></button><button class="icon-button" aria-label="下一页运行记录" :disabled="runPage === runPages" @click="runPage++"><Icon name="ChevronRight" :size="16" /></button></div></footer>
      </section>
      <section class="card log-viewer-card">
        <LogPanel v-if="activeRun" :key="activeRun.id" :run-id="activeRun.id" :title="activeRun.name" embedded />
        <div v-else class="empty-state compact-empty"><div class="empty-icon"><Icon name="Terminal" :size="28" /></div><h3>选择一条运行记录</h3><p>右侧会显示对应的完整运行日志。</p></div>
      </section>
    </div>
  </template>
</template>
