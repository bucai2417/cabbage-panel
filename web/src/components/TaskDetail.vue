<script setup>
import { ref, computed, watch } from 'vue'
import { api, date, statusLabel, usePolling } from '../api'
import Icon from './Icon.vue'
import LogPanel from './LogPanel.vue'

const props = defineProps({ task: { type: Object, required: true }, pending: Boolean })
defineEmits(['back', 'edit', 'source', 'run', 'stop'])

const runs = ref([]), logKey = ref(0), loading = ref(true)
const statusClass = computed(() => props.task.status === 0 ? 'running' : props.task.lastStatus || '')
const statusText = computed(() => props.task.status === 0 ? '运行中' : statusLabel[props.task.lastStatus] || '尚未运行')
const finishedRuns = computed(() => runs.value.filter(run => !['running', 'queued'].includes(run.status)))
const successCount = computed(() => finishedRuns.value.filter(run => run.status === 'success').length)
const failedCount = computed(() => finishedRuns.value.filter(run => !['success', 'cancelled'].includes(run.status)).length)
const successRate = computed(() => {
  const total = successCount.value + failedCount.value
  return total ? Math.round(successCount.value / total * 100) : null
})
const averageDuration = computed(() => {
  if (!finishedRuns.value.length) return '—'
  const total = finishedRuns.value.reduce((sum, run) => sum + Number(run.duration || 0), 0)
  return (total / finishedRuns.value.length).toFixed(2) + 's'
})
async function loadRuns() { runs.value = await api('/logs/runs?kind=crons&taskId=' + props.task.id + '&limit=500'); loading.value = false }
usePolling(loadRuns, 6000)
watch(() => props.task.status, (current, previous) => { if (current === 0 && previous !== 0) logKey.value++ })
</script>
<template>
  <div class="task-detail">
    <button class="back-link" @click="$emit('back')"><Icon name="ChevronLeft" :size="16" />返回任务列表</button>
    <div class="task-overview-row">
      <section class="card task-detail-hero">
        <div class="task-detail-mark" :class="{ running: task.status === 0 }"><Icon :name="task.status === 0 ? 'LoaderCircle' : ['failed', 'timeout', 'interrupted'].includes(task.lastStatus) ? 'CircleAlert' : 'Terminal'" :size="30" :class="{ spin: task.status === 0 }" /></div>
        <div class="task-detail-content">
          <span class="eyebrow">TASK DETAIL</span>
          <div class="task-detail-name"><h1>{{ task.name }}</h1><span class="badge" :class="statusClass"><i />{{ statusText }}</span><span v-if="task.isPinned" class="badge queued"><i />置顶</span></div>
          <p class="task-detail-desc">{{ task.remarks || '暂无备注，点击右侧编辑补充任务说明。' }}</p>
          <div class="task-detail-meta"><div><small>Cron 表达式</small><code>{{ task.schedule || '手动运行' }}</code></div><div><small>执行命令</small><code :title="task.command">{{ task.command }}</code></div></div>
        </div>
        <div class="task-detail-side">
          <div class="task-detail-actions"><button class="button" :class="task.status === 0 ? 'danger' : 'primary'" :disabled="pending" @click="task.status === 0 ? $emit('stop') : $emit('run')"><Icon :name="task.status === 0 ? 'Square' : 'Play'" :size="16" />{{ task.status === 0 ? '停止运行' : '立即运行' }}</button><button class="button" :disabled="pending" @click="$emit('edit')"><Icon name="Pencil" :size="15" />编辑任务</button><button class="button" @click="$emit('source')"><Icon name="FileCode2" :size="15" />源码</button></div>
          <div class="task-detail-times"><div><small>上次运行</small><span>{{ date(task.lastRun) }}</span></div><div><small>下次运行</small><span>{{ task.isDisabled ? '已停用' : date(task.nextRun) }}</span></div></div>
        </div>
      </section>
      <section class="card task-stat-card">
        <div class="card-heading"><h2><Icon name="Activity" :size="18" />执行概览</h2><span class="muted">最近 {{ runs.length }} 次</span></div>
        <div class="task-stat-grid"><div><small>成功率</small><strong>{{ successRate === null ? '—' : successRate }}<i v-if="successRate !== null">%</i></strong></div><div><small>成功</small><strong class="success-text">{{ successCount }}</strong></div><div><small>失败</small><strong class="danger-text">{{ failedCount }}</strong></div><div><small>平均耗时</small><strong>{{ averageDuration }}</strong></div></div>
        <div class="task-progress"><i :style="{ width: (successRate || 0) + '%' }" /></div>
        <div class="task-recent-runs"><div v-for="run in runs.slice(0, 3)" :key="run.id"><span class="badge" :class="run.status"><i />{{ statusLabel[run.status] }}</span><code>#{{ run.id }}</code><span>{{ date(run.started || run.created) }}</span><small>{{ Number(run.duration || 0).toFixed(2) }}s</small></div><p v-if="!runs.length && !loading" class="muted">还没有运行记录，点击「立即运行」开始第一次执行。</p></div>
      </section>
    </div>
    <section class="card task-log-card"><LogPanel :key="logKey" :task-id="task.id" :title="task.name" embedded /></section>
  </div>
</template>
