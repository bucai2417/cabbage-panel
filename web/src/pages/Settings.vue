<script setup>
import { ref, onMounted, onBeforeUnmount, computed, watch, nextTick } from 'vue'
import { api, action, toast, date, bytes, copy } from '../api'
import { confirmDelete, confirmDialog, detailList } from '../dialogs'
import Icon from '../components/Icon.vue'
import FormDialog from '../components/FormDialog.vue'
import NotificationSettings from '../components/NotificationSettings.vue'
const props = defineProps({ version: String, focusTab: String })
const emit = defineEmits(['consume-tab'])
const tab = ref('general'), settings = ref({}), runtimes = ref([]), backups = ref([]), apps = ref([]), loginLogs = ref([]), ipBlacklist = ref([]), notifyLogs = ref([]), systemLog = ref(''), rebuildBusy = ref(false), modal = ref(null), secret = ref(null), saving = ref(false), update = ref(null), runId = ref(0), importing = ref(false), account = ref({ username: '', oldPassword: '', password: '' }), mcpToken = ref(''), mcpHours = ref(2), mcpExpiration = ref(0), mcpAppId = ref(0), mcpTokens = ref([]), mcpTokenId = ref(0), tokenDialog = ref(null), tokenSaving = ref(false), authorization = ref(null), progress = ref(null), restarting = ref(false), runtimeProgress = ref(null), runtimeBusy = ref('')
const about = ref(null), updateDialog = ref(null), checking = ref(false), updating = ref(false), updateStep = ref(''), restartBusy = ref(false)
let progressTimer = null, runtimeTimer = null
const tabs = [{ id: 'general', label: '常规设置', icon: 'SlidersHorizontal' }, { id: 'apps', label: '开放应用 / MCP', icon: 'Plug' }, { id: 'backups', label: '备份恢复', icon: 'DatabaseBackup' }, { id: 'logs', label: '日志', icon: 'ScrollText' }, { id: 'about', label: '关于', icon: 'Info' }, { id: 'security', label: '账户安全', icon: 'ShieldCheck' }]
const generalTab = ref('exec'), logTab = ref('login'), systemLogEl = ref(null)
const generalTabs = [{ id: 'exec', label: '执行与存储' }, { id: 'mirror', label: '依赖镜像' }, { id: 'runtime', label: '运行环境' }, { id: 'source', label: '运行时下载源' }, { id: 'notify', label: '通知' }]
const logTabs = [{ id: 'login', label: '登录日志' }, { id: 'notify', label: '通知日志' }, { id: 'system', label: '系统日志' }]
async function changeGeneralTab(id) {
  if (id === generalTab.value) return
  if (generalTab.value === 'notify' && notificationForm.value && !await notificationForm.value.canLeave()) return
  generalTab.value = id
}
const scopes = ['crons', 'envs', 'scripts', 'logs', 'configs', 'subscriptions', 'dependencies', 'mcp']
const scopeInfo = { crons: '任务与调度', envs: '环境变量', scripts: '脚本文件', logs: '运行日志', configs: '系统配置', subscriptions: '订阅同步', dependencies: '依赖安装', mcp: 'MCP 工具' }
const selectedApps = ref([]), selectedBackups = ref([]), batchBusy = ref(false)
const sortedApps = computed(() => [...apps.value].sort((left, right) => new Date(right.createdAt || 0) - new Date(left.createdAt || 0) || right.id - left.id))
const sortedBackups = computed(() => [...backups.value].sort((left, right) => new Date(right.created || 0) - new Date(left.created || 0) || left.name.localeCompare(right.name)))
function backupPath(item) { return '/backups/' + encodeURIComponent(item.name) + (item.auto ? '?auto=1' : '') }
function backupPathByName(name) { const item = backups.value.find(entry => entry.name === name); return '/backups/' + encodeURIComponent(name) + (item?.auto ? '?auto=1' : '') }
function appExpired(app) {
  const value = String(app.expiresAt || '').trim()
  if (!value) return false
  const deadline = new Date(value.length <= 10 ? value + 'T23:59:59' : value)
  return !Number.isNaN(deadline.getTime()) && deadline < new Date()
}
const mcpApps = computed(() => sortedApps.value.filter(app => (app.scopes || []).includes('mcp') && !appExpired(app)))
watch(mcpApps, list => { if (!list.some(app => app.id === mcpAppId.value)) mcpAppId.value = list[0]?.id || 0 }, { immediate: true })
watch(mcpAppId, appId => { loadMcpTokens(appId) })
async function loadMcpTokens(appId, keepToken = false) {
  mcpTokens.value = []
  mcpTokenId.value = 0
  if (!keepToken) {
    mcpToken.value = ''
    mcpExpiration.value = 0
  }
  if (!appId) return
  try {
    const list = await api('/apps/' + appId + '/tokens')
    const now = Date.now() / 1000
    mcpTokens.value = (list || []).filter(item => item.permanent || Number(item.expires) > now)
    if (mcpTokens.value.length) selectMcpToken(mcpTokens.value[0].id, keepToken)
  } catch { /* 页面其他请求会显示错误 */ }
}
function selectMcpToken(id, keepToken = false) {
  const numeric = Number(id)
  mcpTokenId.value = numeric
  const item = mcpTokens.value.find(token => token.id === numeric)
  // 服务端只保存令牌散列，历史 Authorization 无法再次显示明文。
  if (!keepToken) {
    mcpToken.value = ''
    mcpExpiration.value = item ? Number(item.expires) : 0
  }
}
async function revokeMcpToken() {
  if (!mcpAppId.value || !mcpTokenId.value) return
  if (!await confirmDelete({ title: '撤销当前 Authorization？', message: '使用该令牌的客户端将立即失去访问权限。', confirmText: '撤销令牌' })) return
  await action(async () => { await api('/apps/' + mcpAppId.value + '/tokens/' + mcpTokenId.value, 'DELETE'); await loadMcpTokens(mcpAppId.value) }, 'Authorization 已撤销')
}
async function deleteSelectedApps() {
  if (!selectedApps.value.length) return
  const names = apps.value.filter(item => selectedApps.value.includes(item.id)).map(item => item.name)
  if (!await confirmDelete({
    title: '删除 ' + selectedApps.value.length + ' 个开放应用？',
    message: '应用及其访问凭证会被撤销，使用这些凭证的客户端将立即失去访问权限。',
    details: detailList(names, 4),
    note: '此操作不可撤销，需要重新创建应用并分发新密钥。',
    confirmText: '删除应用',
  })) return
  batchBusy.value = true
  await action(async () => { await api('/apps', 'DELETE', selectedApps.value); selectedApps.value = []; apps.value = await api('/apps') }, '所选应用已删除')
  batchBusy.value = false
}
async function deleteSelectedBackups() {
  if (!selectedBackups.value.length) return
  if (!await confirmDelete({
    title: '永久删除 ' + selectedBackups.value.length + ' 个备份？',
    message: '备份 ZIP 会从磁盘移除，之后无法再用它们恢复数据。',
    details: detailList(selectedBackups.value, 4),
    note: '此操作不可撤销。',
    confirmText: '删除备份',
  })) return
  batchBusy.value = true
  const failures = [], original = [...selectedBackups.value]
  for (const name of original) { try { await api(backupPathByName(name), 'DELETE') } catch (error) { failures.push(name); toast(name + '：' + error.message, 'error') } }
  selectedBackups.value = failures
  await action(async () => { backups.value = await api('/backups') })
  toast('已删除 ' + (original.length - failures.length) + ' 个备份')
  batchBusy.value = false
}
const notificationForm = ref(null)
async function changeTab(value) { if (notificationForm.value && !await notificationForm.value.canLeave()) return; tab.value = value }
const location = window.location
const chosenScopes = ref(['crons', 'logs', 'mcp'])
const mcpConfig = computed(() => JSON.stringify({ mcpServers: { cabbage: { type: 'http', url: location.origin + '/mcp', headers: { Authorization: 'Bearer ' + (mcpToken.value || '<access_token>') } } } }, null, 2))
async function load() { const results = await Promise.allSettled([api('/system/settings'), api('/system/runtimes'), api('/backups'), api('/apps'), api('/user/login-log'), api('/user'), api('/user/ip-blacklist')]); const targets = [settings, runtimes, backups, apps, loginLogs]; results.forEach((result, index) => { if (result.status === 'fulfilled') { if (index < targets.length) targets[index].value = result.value; else if (index === 5) account.value.username = result.value.username; else ipBlacklist.value = result.value } else if (index < 5) toast(result.reason.message, 'error') }) }
async function rebuildPython() {
  if (!await confirmDialog({
    tone: 'danger',
    icon: 'RefreshCw',
    eyebrow: 'PYTHON RUNTIME',
    title: '重建 Python 环境？',
    message: '将删除 data/packages/venv 并用当前配置的 Python 重新创建，随后自动重装所有 Python 依赖。',
    note: '重建期间不能运行脚本。如果旧环境被占用，请先停止正在运行的脚本。',
    confirmText: '开始重建',
    cancelText: '取消',
  })) return
  rebuildBusy.value = true
  try {
    const result = await api('/system/python/rebuild', 'POST')
    toast('已重建：Python ' + (result.version || '') + (result.reinstalled ? '，正在重装 ' + result.reinstalled + ' 个依赖' : ''))
    await load()
  } catch (error) { toast(error.message, 'error') } finally { rebuildBusy.value = false }
}
async function loadBlacklist() { try { ipBlacklist.value = await api('/user/ip-blacklist'); toast(ipBlacklist.value.length ? '当前黑名单：' + ipBlacklist.value.join('、') : 'IP 黑名单为空') } catch (error) { toast(error.message, 'error') } }
async function loadLoginLogs() { try { loginLogs.value = await api('/user/login-log') } catch (error) { toast(error.message, 'error') } }
async function loadNotifyLogs() { try { notifyLogs.value = await api('/system/notify-log') } catch (error) { toast(error.message, 'error') } }
async function loadSystemLog() {
  try {
    systemLog.value = await api('/system/log')
    // 打开即定位到最新日志：等渲染后滚到底部
    await nextTick()
    if (systemLogEl.value) systemLogEl.value.scrollTop = systemLogEl.value.scrollHeight
  } catch (error) { toast(error.message, 'error') }
}
async function clearSystemLog() { if (!await confirmDelete({ title: '清空系统日志？', message: '系统运行日志会被永久删除。', note: '任务运行日志不受影响。', confirmText: '清空' })) return; try { await api('/system/log', 'DELETE'); systemLog.value = ''; toast('系统日志已清空') } catch (error) { toast(error.message, 'error') } }
function loadLogTab() { if (logTab.value === 'login') loadLoginLogs(); else if (logTab.value === 'notify') loadNotifyLogs(); else if (logTab.value === 'system') loadSystemLog() }
watch(tab, value => { if (value === 'logs') loadLogTab(); if (value === 'about') loadAbout() })
onMounted(loadAbout)
watch(logTab, () => loadLogTab())
async function addBlacklist(ip) { try { ipBlacklist.value = await api('/user/ip-blacklist', 'PUT', { ip }); toast('已封禁 ' + ip) } catch (error) { toast(error.message, 'error') } }
async function removeBlacklist(ip) { try { ipBlacklist.value = await api('/user/ip-blacklist', 'DELETE', { ip }); toast('已解封 ' + ip) } catch (error) { toast(error.message, 'error') } }
onMounted(async () => { await load(); try { const state = await api('/system/update/progress'); if (state && state.running) { progress.value = state; stopProgressPoll(); progressTimer = setTimeout(pollUpdateProgress, 300) } } catch { /* 忽略：可能尚未登录或无更新能力 */ } try { const runtimeState = await api('/system/runtimes/progress'); if (runtimeState && runtimeState.running) { runtimeProgress.value = runtimeState; stopRuntimePoll(); runtimeTimer = setTimeout(pollRuntimeProgress, 300) } } catch { /* 忽略：可能尚未登录 */ } })
onBeforeUnmount(() => { stopProgressPoll(); stopRuntimePoll() })
watch(() => props.focusTab, value => { if (value) { tab.value = value; emit('consume-tab') } }, { immediate: true })
async function save(keys) { saving.value = true; await action(() => api('/system/settings', 'PUT', Object.fromEntries(keys.map(key => [key, settings.value[key]]))), '设置已保存'); saving.value = false }
async function createBackup() { saving.value = true; await action(async () => { await api('/backups/export', 'POST'); backups.value = await api('/backups') }, '备份已创建'); saving.value = false }
async function importBackup(event) { const file = event.target.files?.[0]; event.target.value = ''; if (!file) return; if (!await confirmDialog({ tone: 'danger', icon: 'DatabaseBackup', eyebrow: '恢复备份', title: '导入并覆盖当前数据？', message: '任务、变量、脚本、应用和账号都会被备份中的内容替换。', details: [file.name], note: '系统会先创建恢复前备份；恢复后需要重新登录。', confirmText: '继续导入', cancelText: '取消' })) return; importing.value = true; const form = new FormData(); form.append('file', file); await action(async () => { await api('/backups/import', 'POST', form); window.dispatchEvent(new Event('session-expired')) }, '恢复成功，请使用备份中的账户重新登录'); importing.value = false }
async function deleteBackup(name) { if (!await confirmDelete({ title: '永久删除此备份？', message: '备份 ZIP 会从磁盘移除，之后无法再用它恢复数据。', details: [name], confirmText: '删除备份' })) return; await action(async () => { await api(backupPathByName(name), 'DELETE'); backups.value = await api('/backups') }, '备份已删除') }
async function saveApp(values) { saving.value = true; try { const result = await api('/apps', values.id ? 'PUT' : 'POST', { ...values, scopes: chosenScopes.value }); modal.value = null; if (result.client_secret) secret.value = result; apps.value = await api('/apps'); toast('应用已保存') } catch (error) { toast(error.message, 'error') } finally { saving.value = false } }
function editApp(item) { chosenScopes.value = item ? [...item.scopes] : ['crons', 'logs', 'mcp']; modal.value = item ? { ...item, expiresAt: (item.expiresAt || '').slice(0, 10) } : { name: '', expiresAt: '' } }
async function removeApp(item) { if (!await confirmDelete({ title: '删除应用「' + item.name + '」？', message: '应用的全部访问凭证会被撤销，使用这些凭证的客户端将立即失去访问权限。', details: [item.name, item.client_id], note: '此操作不可撤销。', confirmText: '删除应用' })) return; await action(async () => { await api('/apps', 'DELETE', [item.id]); apps.value = await api('/apps') }, '应用已删除') }
async function resetSecret(item) { if (!await confirmDialog({ tone: 'warning', icon: 'KeyRound', eyebrow: '重置密钥', title: '重置「' + item.name + '」的密钥？', message: '已有 token 会立即失效，自动化客户端需要使用新密钥重新获取凭证。', details: [item.client_id], confirmText: '重置密钥', cancelText: '取消' })) return; await action(async () => { secret.value = await api('/apps/' + item.id + '/reset-secret', 'PUT') }, '密钥已重置') }
async function generateMcpToken() {
  const app = mcpApps.value.find(item => item.id === mcpAppId.value)
  if (!app) return toast('请选择一个带 mcp 权限且未过期的应用', 'error')
  const hours = Number(mcpHours.value)
  const payload = hours === 0 ? { permanent: true } : { expiresIn: Math.round(Math.max(1, Math.min(720, hours)) * 3600) }
  await action(async () => {
    const result = await api('/apps/' + app.id + '/token', 'POST', payload)
    mcpToken.value = result.token
    mcpExpiration.value = result.expiration
    await loadMcpTokens(app.id, true)
  }, 'Authorization 已生成')
}
function openAuthorization(item) { tokenDialog.value = item }
async function createAuthorization(values) {
  if (!tokenDialog.value || tokenSaving.value) return
  tokenSaving.value = true
  const app = tokenDialog.value
  try {
    const hours = Math.max(1, Math.min(720, Number(values.hours || 24)))
    const payload = values.permanent ? { permanent: true } : { expiresIn: Math.round(hours * 3600) }
    const result = await api('/apps/' + app.id + '/token', 'POST', payload)
    authorization.value = { ...result, name: app.name }
    if ((app.scopes || []).includes('mcp')) {
      mcpAppId.value = app.id
      await loadMcpTokens(app.id)
    }
    tokenDialog.value = null
  } catch (error) { toast(error.message, 'error') } finally { tokenSaving.value = false }
}
async function checkUpdate() {
  if (checking.value) return
  checking.value = true
  try {
    const release = await api('/system/update')
    if (release && typeof release.latest === 'string') release.latest = release.latest.replace(/^v/i, '')
    update.value = release
    if (release.available) { updateDialog.value = release; updateStep.value = '' }
    else toast('已是最新版本 v' + release.current)
  } catch (error) { toast(error.message, 'error') } finally { checking.value = false }
}
async function downloadAndApply() {
  if (!updateDialog.value || updating.value) return
  if (!await confirmDialog({
    tone: 'warning', icon: 'RefreshCw', eyebrow: '版本更新',
    title: '更新到 v' + updateDialog.value.latest + '？',
    message: '将下载新版本程序，校验后替换并自动重启面板。',
    note: '更新期间请不要关闭页面；重启后页面会自动刷新。',
    confirmText: '开始更新', cancelText: '取消',
  })) return
  updating.value = true
  updateStep.value = '正在下载'
  try {
    await api('/system/update/download', 'POST')
    let state = { running: true, done: false, phase: '准备下载', received: 0, total: 0, percent: 0 }
    progress.value = state
    for (let attempt = 0; attempt < 900; attempt++) {
      await sleep(700)
      state = await api('/system/update/progress')
      progress.value = state
      if (state.error) throw new Error(state.error)
      if (state.done) break
      if (!state.running) break
    }
    if (!state.done) throw new Error('下载超时，请重试')
    updateStep.value = '正在替换并重启'
    await api('/system/update/apply', 'POST')
    updateDialog.value = null
    restarting.value = true
    await waitForRestart()
  } catch (error) {
    toast('更新失败：' + error.message, 'error')
    updating.value = false
    updateStep.value = ''
  }
}
async function restartPanel() {
  if (restartBusy.value) return
  if (!await confirmDialog({
    tone: 'warning', icon: 'RefreshCw', eyebrow: '重启面板',
    title: '重启面板服务？',
    message: '面板会退出并自动重新启动。',
    note: '正在运行的任务会被中断，请先在定时任务页停止。重启后页面会自动刷新。',
    confirmText: '立即重启', cancelText: '取消',
  })) return
  restartBusy.value = true
  try {
    await api('/system/restart', 'POST')
    restarting.value = true
    await waitForRestart()
  } catch (error) { toast(error.message, 'error'); restartBusy.value = false }
}
async function loadAbout() { try { about.value = await api('/system/about') } catch { /* 忽略：未登录或旧版本 */ } }
function updatePercent(item) { if (!item) return 0; if (item.total > 0) return Math.min(100, Math.round(item.received * 100 / item.total)); return item.percent > 0 ? Math.min(100, item.percent) : 0 }
function stopProgressPoll() { if (progressTimer) { clearTimeout(progressTimer); progressTimer = null } }
function stopRuntimePoll() { if (runtimeTimer) { clearTimeout(runtimeTimer); runtimeTimer = null } }
async function installRuntime(name) {
  if (!await confirmDialog({
    tone: 'warning',
    icon: 'Download',
    eyebrow: 'RUNTIME INSTALL',
    title: '安装 ' + name + '？',
    message: '将从官方源下载便携运行时并解压到 data/packages/runtime，不修改系统环境、不写入注册表。',
    note: '安装期间不能运行脚本。已安装的运行时会在校验通过后被原子替换。',
    confirmText: '开始安装',
    cancelText: '取消',
  })) return
  runtimeBusy.value = name
  try {
    await api('/system/runtimes/install', 'POST', { name })
    runtimeProgress.value = { running: true, done: false, runtime: name, phase: '准备安装', received: 0, total: 0, percent: 0 }
    stopRuntimePoll()
    runtimeTimer = setTimeout(pollRuntimeProgress, 300)
    toast('已开始安装 ' + name)
  } catch (error) { toast(error.message, 'error') } finally { runtimeBusy.value = '' }
}
async function pollRuntimeProgress() {
  runtimeTimer = null
  try {
    const state = await api('/system/runtimes/progress')
    runtimeProgress.value = state
    if (state.running) { runtimeTimer = setTimeout(pollRuntimeProgress, 500); return }
    if (state.error) toast(state.runtime + ' 安装失败：' + state.error, 'error')
    else if (state.done) { await load(); toast(state.runtime + ' 已安装就绪') }
  } catch { /* 服务可能正在重启，忽略 */ }
}
async function pollUpdateProgress() { progressTimer = null; try { const state = await api('/system/update/progress'); progress.value = state; if (state.running) { progressTimer = setTimeout(pollUpdateProgress, 500); return } if (state.error) toast('更新下载失败：' + state.error, 'error'); else if (state.done) { await checkUpdate(); if (state.version) toast('已下载并校验 v' + state.version) } } catch { /* 服务可能正在重启，忽略 */ } }
function sleep(ms) { return new Promise(resolve => setTimeout(resolve, ms)) }
async function waitForRestart() { let wentDown = false; for (let attempt = 0; attempt < 90; attempt++) { await sleep(1000); try { const state = await api('/system/init'); if (state && wentDown) { location.reload(); return } } catch { wentDown = true } } location.reload() }


async function updateAccount() { saving.value = true; await action(async () => { await api('/user', 'PUT', account.value); window.dispatchEvent(new Event('session-expired')) }, '账户已更新，请重新登录'); saving.value = false }
</script>
<template>
  <div class="settings-layout"><aside class="settings-nav"><button v-for="item in tabs" :key="item.id" :class="{ active: tab === item.id }" @click="changeTab(item.id)"><Icon :name="item.icon" :size="18" />{{ item.label }}</button></aside><div class="settings-content">
    <template v-if="tab === 'general'"><div class="settings-subtabs"><button v-for="item in generalTabs" :key="item.id" :class="{ active: generalTab === item.id }" @click="changeGeneralTab(item.id)">{{ item.label }}</button></div><section v-if="generalTab === 'exec'" class="card settings-card"><div class="card-heading"><h2><Icon name="SlidersHorizontal" />执行与存储</h2></div><form @submit.prevent="save(['concurrency', 'timeout', 'retentionDays'])"><div class="settings-fields"><label class="field"><span>全局最大并发数</span><input v-model="settings.concurrency" type="number" min="1" max="32" required /><small>1–32 个后台任务；超出部分自动排队，修改立即生效。</small></label><label class="field"><span>默认执行超时（秒）</span><input v-model="settings.timeout" type="number" min="1" max="604800" required /><small>任务可单独设置超时，超时后清理进程树。</small></label><label class="field"><span>日志保留天数</span><input v-model="settings.retentionDays" type="number" min="1" max="3650" required /><small>每天自动清理过期运行记录和日志。</small></label></div><footer><button type="button" class="button" @click="action(() => api('/system/cleanup', 'PUT'), '已清理过期日志')"><Icon name="Brush" :size="17" />立即清理</button><button class="button primary" :disabled="saving"><Icon name="Check" :size="17" />保存设置</button></footer></form></section><section v-else-if="generalTab === 'mirror'" class="card settings-card"><div class="card-heading"><h2><Icon name="Package" />依赖安装镜像</h2><span class="badge">可选</span></div><form @submit.prevent="save(['npmMirror', 'pythonMirror', 'linuxMirror', 'autoInstallDeps'])"><div class="settings-fields"><label class="field"><span>JavaScript / Node.js 镜像源（npm）</span><input v-model="settings.npmMirror" placeholder="https://registry.npmmirror.com" /><small>Node.js / JavaScript / TypeScript 依赖安装时使用；留空使用 npm 默认源。</small></label><label class="field"><span>Python 镜像源（pip）</span><input v-model="settings.pythonMirror" placeholder="https://pypi.tuna.tsinghua.edu.cn/simple" /><small>pip 安装时通过 -i 指定；留空使用 PyPI 默认源。</small></label><label class="field"><span>Linux 软件包镜像源（apt / apk）</span><input v-model="settings.linuxMirror" placeholder="https://mirrors.tuna.tsinghua.edu.cn/ubuntu/" /><small>仅 Linux 生效：apt 通过临时 sources.list 使用，apk 通过 --repository 使用；留空使用系统默认源。</small></label><label class="field"><span>任务失败后自动安装缺失依赖</span><select v-model="settings.autoInstallDeps"><option value="false">关闭（推荐）</option><option value="true">开启</option></select><small>开启后会从任务日志中提取 npm / pip 包名并自动安装。日志内容由运行的脚本决定，等于信任脚本能触发任意包安装，请只在运行可信脚本时开启。</small></label></div><footer><button class="button primary" :disabled="saving"><Icon name="Check" :size="17" />保存设置</button></footer></form><p class="settings-footnote">镜像地址需为 HTTP(S) URL，修改后对后续依赖安装立即生效，不影响已安装的包。apt 镜像会自动识别系统发行版代号（如 bookworm / jammy）拼接源地址。</p></section><section v-else-if="generalTab === 'runtime'" class="card settings-card"><div class="card-heading"><h2><Icon name="Cpu" />运行环境</h2><span class="badge success">端口 5700</span></div><form @submit.prevent="save(['pythonPath', 'pythonVersion'])"><div class="settings-fields"><label class="field"><span>Python 解释器路径（可选）</span><input v-model="settings.pythonPath" placeholder="留空则自动检测，如 /usr/bin/python3.10" spellcheck="false" /><small>绝对路径优先于版本号；填写后需重建 Python 环境才生效。</small></label><label class="field"><span>Python 版本（可选）</span><input v-model="settings.pythonVersion" placeholder="3.11" spellcheck="false" /><small>按 python3.11 → python3 → python 顺序查找；留空使用系统默认。</small></label></div><footer><button class="button primary" :disabled="saving"><Icon name="Check" :size="17" />保存配置</button><button type="button" class="button" :disabled="rebuildBusy || saving" @click="rebuildPython"><Icon :name="rebuildBusy ? 'LoaderCircle' : 'RefreshCw'" :size="17" :class="{ spin: rebuildBusy }" />{{ rebuildBusy ? '重建中…' : '重建 Python 环境' }}</button></footer></form><div class="runtime-list"><div v-for="runtime in runtimes" :key="runtime.name" :class="{ 'runtime-missing': !runtime.available }"><span class="resource-icon"><Icon name="Terminal" /></span><div><strong>{{ runtime.name }}<template v-if="runtime.version"> <small class="runtime-version">{{ runtime.version }}</small></template><small v-if="runtime.source === 'portable'" class="runtime-portable">便携</small></strong><code>{{ runtime.path || (runtime.installHint || '请安装并添加到 PATH') }}</code><small v-if="runtime.source === 'configured-fallback'" class="runtime-warn">配置不可用，已回退到系统默认</small></div><div class="runtime-actions"><span class="badge" :class="runtime.available ? 'success' : 'failed'">{{ runtime.available ? (runtime.source === 'portable' ? '便携运行时' : runtime.source === 'configured-path' ? '自定义路径' : runtime.source === 'configured-version' ? '自定义版本' : '可用') : '未检测到' }}</span><button v-if="!runtime.available && runtime.installable" type="button" class="button compact" :disabled="!!runtimeBusy || (runtimeProgress && runtimeProgress.running)" @click="installRuntime(runtime.name)"><Icon :name="runtimeBusy === runtime.name ? 'LoaderCircle' : 'Download'" :size="15" :class="{ spin: runtimeBusy === runtime.name }" />安装</button></div></div></div><div v-if="runtimeProgress" class="runtime-progress"><div class="update-progress"><div class="update-progress-bar"><span :style="{ width: updatePercent(runtimeProgress) + '%' }" /></div><div class="update-progress-meta"><strong>{{ runtimeProgress.runtime }} · {{ updatePercent(runtimeProgress) }}%</strong><span v-if="runtimeProgress.total > 0">{{ bytes(runtimeProgress.received) }} / {{ bytes(runtimeProgress.total) }}</span><span v-else-if="runtimeProgress.received">{{ bytes(runtimeProgress.received) }}</span><span v-else>{{ runtimeProgress.phase }}</span></div></div><p v-if="runtimeProgress.error" class="inline-error" role="alert">{{ runtimeProgress.error }}</p><p v-else-if="runtimeProgress.running" class="muted">{{ runtimeProgress.phase }}…安装期间请勿运行脚本。</p></div><p class="settings-footnote">依赖不随程序打包：Python 依赖装在 data/packages/venv，Node 依赖装在 data/packages/node_modules，运行脚本需要宿主机提供 Python / Node。缺失时可点「安装」下载便携运行时到 data/packages/runtime，无需管理员权限。切换 Python 版本后请点「重建 Python 环境」并等待依赖自动重装。</p></section><section v-else-if="generalTab === 'source'" class="card settings-card"><div class="card-heading"><h2><Icon name="Download" />运行时下载源</h2><span class="badge">可选</span></div><form @submit.prevent="save(['runtimePythonMirror', 'runtimeNodeMirror', 'runtimeGitMirror', 'runtimeMambaMirror', 'runtimeMambaChannel'])"><div class="settings-fields"><label class="field"><span>Python 运行时下载基址</span><input v-model="settings.runtimePythonMirror" placeholder="留空使用内置源（npmmirror python-build-standalone）" spellcheck="false" /><small>只需填到目录，程序会自动拼接包文件名。</small></label><label class="field"><span>Node.js 运行时下载基址</span><input v-model="settings.runtimeNodeMirror" placeholder="留空使用内置源（npmmirror Node）" spellcheck="false" /><small>国内网络较慢时可换成其他镜像目录。</small></label><label class="field"><span>Git 运行时下载基址（Windows）</span><input v-model="settings.runtimeGitMirror" placeholder="留空使用内置源（npmmirror git-for-windows）" spellcheck="false" /><small>Windows 使用 MinGit 便携包。</small></label><label class="field"><span>micromamba 下载基址（Linux / macOS）</span><input v-model="settings.runtimeMambaMirror" placeholder="留空使用内置源（GitHub micromamba-releases）" spellcheck="false" /><small>Linux / macOS 安装 Git 时先用 micromamba 创建隔离环境，首次会额外下载约 7–9 MB。</small></label><label class="field"><span>conda 频道（Linux / macOS 安装 Git）</span><input v-model="settings.runtimeMambaChannel" placeholder="留空使用 conda-forge，可填国内镜像 URL" spellcheck="false" /><small>国内可换成清华、北外等 conda 镜像地址以加速依赖下载。</small></label></div><footer><button class="button primary" :disabled="saving"><Icon name="Check" :size="17" />保存下载源</button></footer></form><p class="settings-footnote">只覆盖下载地址，包内容仍会用内置 SHA-256 校验；校验不通过的包不会被安装。Linux / macOS 的 Git 通过 micromamba 装入 data/packages/runtime/Git，无需 root，但会一并拉取 conda-forge 的依赖。Docker 部署请直接在镜像内预装运行时。</p></section><NotificationSettings v-else-if="generalTab === 'notify'" ref="notificationForm" /></template>
    <template v-else-if="tab === 'apps'"><section class="card settings-card"><div class="card-heading"><h2><Icon name="KeyRound" />开放应用</h2><button class="button primary compact" @click="editApp()"><Icon name="Plus" :size="17" />创建应用</button></div><div class="app-list-toolbar"><label class="inline-check"><input type="checkbox" aria-label="选择全部应用" :checked="apps.length > 0 && apps.every(item => selectedApps.includes(item.id))" @change="selectedApps = $event.target.checked ? apps.map(item => item.id) : []" />选择全部</label><span class="muted">{{ sortedApps.length }} 个应用</span></div><div class="app-list"><div v-for="item in sortedApps" :key="item.id" class="app-row" :class="{ 'selected-row': selectedApps.includes(item.id), expired: appExpired(item) }"><input v-model="selectedApps" type="checkbox" :value="item.id" :aria-label="'选择应用 ' + item.name" /><div class="app-main"><div class="app-title"><strong>{{ item.name }}</strong><span v-if="appExpired(item)" class="badge failed"><i />已过期</span><span v-else-if="item.expiresAt" class="badge queued"><i />至 {{ item.expiresAt }}</span><span v-else class="badge"><i />永久</span></div><code class="app-client">Client ID · {{ item.client_id }}</code><div class="labels"><i v-for="scope in item.scopes" :key="scope">{{ scope }}</i></div></div><div class="app-actions"><button class="icon-button" aria-label="编辑权限" title="编辑" @click="editApp(item)"><Icon name="Pencil" :size="16" /></button><button class="icon-button" aria-label="创建 Authorization" title="创建 Authorization" :disabled="appExpired(item)" @click="openAuthorization(item)"><Icon name="ShieldCheck" :size="16" /></button><button class="icon-button" aria-label="重置应用密钥" title="重置密钥" @click="resetSecret(item)"><Icon name="KeyRound" :size="16" /></button><button class="icon-button danger" aria-label="删除应用" title="删除应用" @click="removeApp(item)"><Icon name="Trash2" :size="16" /></button></div></div><p v-if="!sortedApps.length" class="settings-footnote">创建一个应用，为它分配需要访问的接口权限。Client Secret 只在创建或重置时显示一次。</p></div><div v-if="selectedApps.length" class="bulk-bar"><span>已选 {{ selectedApps.length }} 个应用</span><button class="danger-text" :disabled="batchBusy" @click="deleteSelectedApps">删除所选应用</button><button @click="selectedApps = []">取消选择</button></div><p v-if="!apps.length" class="settings-footnote">创建一个应用，为它分配需要访问的接口权限。Client Secret 只在创建或重置时显示一次，数据库仅保存散列。</p></section><section class="card settings-card"><div class="card-heading"><h2><Icon name="Plug" />MCP 服务</h2><span class="badge success"><i />已开放 · 同端口</span></div><div class="settings-fields"><p class="muted">连接地址 <code>{{ location?.origin || '' }}/mcp</code>。使用 Streamable HTTP；应用需要 mcp 权限和对应工具的数据权限。</p><div class="info-banner"><Icon name="Info" /><span>MCP 只需要 Authorization（Bearer）。Client ID / Secret 仅用于标识应用；请选择一个带 mcp 权限且未过期的应用生成令牌。</span></div><form class="token-form" @submit.prevent="generateMcpToken"><label class="field"><span>MCP 应用</span><select v-model="mcpAppId" aria-label="选择 MCP 应用"><option :value="0">请选择应用</option><option v-for="app in mcpApps" :key="app.id" :value="app.id">{{ app.name }}</option></select><small v-if="!mcpApps.length">没有带 mcp 权限且未过期的应用，请先在开放应用中创建。</small></label><label class="field"><span>Authorization 有效期</span><select v-model="mcpHours" aria-label="Authorization 有效期"><option :value="1">1 小时</option><option :value="2">2 小时</option><option :value="6">6 小时</option><option :value="24">24 小时</option><option :value="72">3 天</option><option :value="168">7 天</option><option :value="720">30 天</option><option :value="0">永久</option></select></label><button class="button" :disabled="!mcpAppId">生成 Authorization</button></form><label v-if="mcpTokens.length" class="field"><span>已有 Authorization</span><select v-model="mcpTokenId" aria-label="选择已有 Authorization" @change="selectMcpToken(mcpTokenId)"><option v-for="item in mcpTokens" :key="item.id" :value="item.id">#{{ item.id }} · {{ item.permanent ? '永久' : '至 ' + date(new Date(item.expires * 1000)) }}</option></select><small>数据库只保存令牌散列，历史 Authorization 无法再次显示；需要新令牌请重新生成。</small></label><div v-if="mcpToken" class="mcp-token-box"><label class="field"><span>当前 Authorization</span><textarea :value="mcpToken" readonly rows="3" aria-label="当前 Authorization" /></label><div class="actions"><button class="button compact" @click="copy(mcpToken)"><Icon name="Copy" :size="15" />复制令牌</button><button class="button danger compact" @click="revokeMcpToken"><Icon name="Trash2" :size="15" />撤销令牌</button></div></div><p v-if="mcpToken" class="mcp-token-expiry">当前 Authorization：{{ mcpExpiration ? '有效期至 ' + date(new Date(mcpExpiration * 1000)) : '永久有效' }}</p><pre class="config-code">{{ mcpConfig }}</pre><button class="button" @click="copy(mcpConfig)"><Icon name="Copy" :size="16" />复制 MCP 配置</button></div></section></template>
    <template v-else-if="tab === 'backups'"><section class="card settings-card"><div class="card-heading"><h2><Icon name="Clock3" />自动备份</h2><span class="badge" :class="settings.autoBackupEnabled === 'true' ? 'success' : ''">{{ settings.autoBackupEnabled === 'true' ? '已开启 · 每天 ' + settings.autoBackupTime : '未开启' }}</span></div><form @submit.prevent="save(['autoBackupEnabled', 'autoBackupTime'])"><div class="settings-fields"><label class="field"><span>每日自动备份</span><select v-model="settings.autoBackupEnabled"><option value="false">关闭</option><option value="true">开启</option></select><small>开启后每天在指定时间创建一致性备份，自动新建 data/backups/auto 目录保存文件，并仅保留最近 14 份。</small></label><label class="field"><span>备份时间</span><input v-model="settings.autoBackupTime" type="time" required /><small>使用服务器本地时间。到点时如有任务或请求正在执行，本次自动备份会跳过并写入系统日志，次日继续。</small></label></div><footer><button class="button primary" :disabled="saving"><Icon name="Check" :size="17" />保存自动备份</button></footer></form></section><section class="card settings-card"><div class="card-heading"><h2><Icon name="DatabaseBackup" />备份与恢复</h2></div><div class="settings-fields"><p class="muted">备份包括任务、环境变量、应用、账号、配置、脚本与最近 10000 条运行记录。依赖二进制不随备份迁移，恢复后重新安装即可。</p><div class="info-banner"><Icon name="LockKeyhole" /><span>备份包含敏感变量和账号散列，请妥善保存。为保证一致性，创建和恢复时需要队列清空。</span></div><div class="actions"><button class="button primary" :disabled="saving || importing" @click="createBackup"><Icon name="Download" :size="17" />{{ saving ? '正在备份…' : '创建备份' }}</button><label class="button upload-button" :class="{ disabled: importing }"><Icon name="Upload" :size="17" />{{ importing ? '正在恢复…' : '导入备份' }}<input type="file" accept=".zip" :disabled="importing" @change="importBackup" /></label></div></div><div class="table-scroll"><table><thead><tr><th class="checkbox-column"><input type="checkbox" aria-label="选择全部备份" :checked="backups.length > 0 && backups.every(item => selectedBackups.includes(item.name))" @change="selectedBackups = $event.target.checked ? backups.map(item => item.name) : []" /></th><th>备份文件</th><th>大小</th><th>时间</th><th>操作</th></tr></thead><tbody><tr v-for="item in sortedBackups" :key="item.name" :class="{ 'selected-row': selectedBackups.includes(item.name) }"><td><input v-model="selectedBackups" type="checkbox" :value="item.name" :aria-label="'选择备份 ' + item.name" /></td><td><code>{{ item.name }}</code><span v-if="item.auto" class="badge queued backup-auto">自动</span></td><td>{{ bytes(item.size) }}</td><td>{{ date(item.created) }}</td><td><div class="actions"><a class="icon-button" :href="'/api' + backupPath(item)" download aria-label="下载备份"><Icon name="Download" :size="17" /></a><button class="icon-button danger" aria-label="删除备份" @click="deleteBackup(item.name)"><Icon name="Trash2" :size="17" /></button></div></td></tr></tbody></table></div><div v-if="selectedBackups.length" class="bulk-bar"><span>已选 {{ selectedBackups.length }} 个备份</span><button class="danger-text" :disabled="batchBusy || importing" @click="deleteSelectedBackups">删除所选备份</button><button @click="selectedBackups = []">取消选择</button></div><p v-if="!backups.length" class="settings-footnote">还没有备份，建议在批量修改或更新前创建一份。</p></section></template>
    <template v-else-if="tab === 'about'"><section class="card settings-card about-card"><div class="about-hero"><img src="/logo.png" alt="大白菜面板" class="about-logo" /><div class="about-heading"><h2>大白菜面板 <span class="about-version">v{{ version }}</span></h2><p>轻量、自托管的自动化任务面板 · Go + Vue 3 + SQLite</p></div></div><div class="about-body"><p class="about-intro">把定时任务、脚本管理、依赖安装、通知、备份与 MCP 开放接口收进一个单文件程序。默认端口 5700，网页、管理 API、开放 API 与 MCP 共用同一端口，数据全部保存在本地，自主可控。</p><dl class="about-meta"><div><dt>作者</dt><dd>bucai</dd></div><div><dt>版本</dt><dd>v{{ version }}</dd></div><div><dt>操作系统</dt><dd>{{ about?.osName || '读取中…' }}<span v-if="about?.arch" class="about-arch">&nbsp;· {{ about.arch }}</span></dd></div><div><dt>运行环境</dt><dd>{{ about?.go || '—' }}<span v-if="about?.platform" class="about-arch">&nbsp;· {{ about.platform }}</span></dd></div><div><dt>主机名</dt><dd>{{ about?.hostname || '—' }}</dd></div><div><dt>许可证</dt><dd>Apache-2.0 + Commons Clause</dd></div><div class="about-meta-wide"><dt>仓库</dt><dd><a href="https://github.com/bucai2417/cabbage-panel" target="_blank" rel="noopener noreferrer"><Icon name="GitBranch" :size="14" />github.com/bucai2417/cabbage-panel</a></dd></div></dl></div></section><section class="card settings-card"><div class="card-heading"><h2><Icon name="RefreshCw" />版本更新</h2><span class="badge" :class="about?.signedUpdates ? 'success' : (settings.allowUnsignedUpdates === 'true' ? 'queued' : 'failed')">{{ about?.signedUpdates ? '签名校验' : (settings.allowUnsignedUpdates === 'true' ? '未签名 · 已允许' : '未签名 · 已禁用') }}</span></div><div class="settings-fields"><div v-if="about && !about.signedUpdates" class="info-banner danger"><Icon name="CircleAlert" /><span>当前构建未嵌入发布公钥，更新清单仅靠 HTTPS 传输，无法抵御仓库或 Release 被篡改。默认已禁用更新；只有完全信任更新源时才开启「允许未签名更新」。</span></div><label class="field"><span>更新源</span><input v-model="settings.githubRepo" placeholder="owner/repo" /><small>默认 bucai2417/cabbage-panel；仅用于检查更新，可改成你自己的发布仓库。</small></label><label v-if="about && !about.signedUpdates" class="field"><span>允许未签名更新</span><select v-model="settings.allowUnsignedUpdates"><option value="false">关闭（推荐）</option><option value="true">开启（仅校验 SHA-256）</option></select><small>开启后，任何能篡改 GitHub Release 的人都能让面板下载并执行任意二进制。</small></label><div class="about-actions"><button class="button" :disabled="saving" @click="save(about?.signedUpdates ? ['githubRepo'] : ['githubRepo', 'allowUnsignedUpdates'])"><Icon name="Check" :size="17" />保存设置</button><button class="button primary" :disabled="saving || checking" @click="checkUpdate"><Icon :name="checking ? 'LoaderCircle' : 'RefreshCw'" :size="17" :class="{ spin: checking }" />{{ checking ? '正在检查…' : '检查更新' }}</button></div></div></section><section class="card settings-card"><div class="card-heading"><h2><Icon name="Power" />面板控制</h2></div><div class="settings-fields"><p class="muted">重启会重新加载面板程序；正在运行的任务请先在定时任务页停止。</p><button class="button warning" :disabled="restartBusy" @click="restartPanel"><Icon :name="restartBusy ? 'LoaderCircle' : 'RefreshCw'" :size="17" :class="{ spin: restartBusy }" />{{ restartBusy ? '正在重启…' : '重启面板' }}</button></div></section></template>
    <template v-else-if="tab === 'security'"><section class="card settings-card"><div class="card-heading"><h2><Icon name="ShieldCheck" />管理员账户</h2></div><form @submit.prevent="updateAccount"><div class="settings-fields"><label class="field"><span>用户名</span><input v-model="account.username" required minlength="2" autocomplete="username" /></label><label class="field"><span>当前密码</span><input v-model="account.oldPassword" type="password" required autocomplete="current-password" /></label><label class="field"><span>新密码</span><input v-model="account.password" type="password" required minlength="6" maxlength="72" autocomplete="new-password" /><small>至少 6 个字符。保存后撤销全部登录和应用 token。</small></label></div><footer><button class="button primary" :disabled="saving">更新账户</button></footer></form></section><section class="card settings-card"><div class="card-heading"><h2><Icon name="Link" />可信反向代理与出站白名单</h2><span class="badge">可选</span></div><form @submit.prevent="save(['trustedProxies', 'outboundAllowlist', 'ipLocationEnabled', 'ipLocationURL'])"><div class="settings-fields"><label class="field"><span>可信代理地址</span><input v-model="settings.trustedProxies" placeholder="例如：127.0.0.1, 10.0.0.0/8" spellcheck="false" /><small>只有来自这些 IP 或 CIDR 的请求才会解析 X-Forwarded-For 与 X-Forwarded-Proto。面板直接暴露公网时请留空，避免伪造来源 IP 绕过黑名单与限流。过宽网段（IPv4 短于 /8、IPv6 短于 /16）会被拒绝。</small></label><label class="field"><span>出站白名单</span><input v-model="settings.outboundAllowlist" placeholder="例如：192.168.1.10:9000" spellcheck="false" /><small>默认阻止访问回环、私网和云元数据地址；如确需通知到局域网 webhook，可在此填写 host 或 host:port，多个用逗号分隔。</small></label><label class="field"><span>登录 IP 归属地查询</span><select v-model="settings.ipLocationEnabled"><option value="true">开启（HTTPS 第三方接口）</option><option value="false">关闭（不向第三方发送 IP）</option></select><small>仅对登录日志中的公网 IP 查询归属地，内网与回环地址不会外发。隐私敏感部署建议关闭。</small></label><label class="field"><span>归属地接口（可选）</span><input v-model="settings.ipLocationURL" placeholder="留空使用 https://ipwho.is/{ip}?lang=zh-CN" spellcheck="false" /><small>自建接口可填写模板，{ip} 会替换为目标地址；必须是 HTTP(S) URL。接口需返回 ipwho.is 或 ip-api 兼容字段。</small></label></div><footer><button class="button primary" :disabled="saving"><Icon name="Check" :size="17" />保存网络设置</button></footer></form></section></template>
    <template v-else-if="tab === 'logs'"><div class="settings-subtabs"><button v-for="item in logTabs" :key="item.id" :class="{ active: logTab === item.id }" @click="logTab = item.id">{{ item.label }}</button></div><section v-if="logTab === 'login'" class="card settings-card"><div class="card-heading"><h2><Icon name="ScrollText" />登录日志</h2><div class="actions"><button class="small-button" @click="loadBlacklist"><Icon name="Ban" :size="15" />IP 黑名单<span v-if="ipBlacklist.length"> · {{ ipBlacklist.length }}</span></button><button class="icon-button" aria-label="刷新登录日志" @click="action(async () => loginLogs = await api('/user/login-log'))"><Icon name="RefreshCw" /></button></div></div><div class="table-scroll"><table><thead><tr><th class="index-column">序号</th><th>登录时间</th><th>登录地址</th><th>登录 IP</th><th>登录设备</th><th>登录状态</th><th class="align-right">操作</th></tr></thead><tbody><tr v-for="(item, index) in loginLogs" :key="item.ip + '-' + item.timestamp"><td class="muted">{{ index + 1 }}</td><td class="nowrap">{{ date(item.timestamp) }}</td><td>{{ item.address || '—' }}</td><td><code>{{ item.ip || '—' }}</code></td><td>{{ item.platform === 'mobile' ? '移动端' : item.platform === 'desktop' ? '桌面端' : (item.platform || '—') }}</td><td><span class="badge" :class="item.status === 0 ? 'success' : 'failed'">{{ item.status === 0 ? '成功' : '失败' }}</span></td><td class="align-right"><button v-if="item.status === 1 && item.ip && !ipBlacklist.includes(item.ip)" class="text-button danger-text" @click="addBlacklist(item.ip)">加入黑名单</button><span v-else-if="item.ip && ipBlacklist.includes(item.ip)" class="muted">已封禁</span><span v-else class="muted">—</span></td></tr><tr v-if="!loginLogs.length"><td colspan="7" class="muted empty-row">暂无登录记录</td></tr></tbody></table></div><div v-if="ipBlacklist.length" class="settings-fields"><p class="muted">当前黑名单（命中后包括管理员在内的所有请求都会被拒绝）</p><div class="blacklist-list"><div v-for="entry in ipBlacklist" :key="entry">{{ entry }}<button type="button" class="text-button danger-text" @click="removeBlacklist(entry)">移除</button></div></div></div><p class="settings-footnote">字段与青龙 GET /api/user/login-log 一致：timestamp、address、ip、platform、status（0 成功、1 失败），最多保留最近 100 条。不记录请求体、密码和变量内容。保存黑名单前会校验当前访问 IP，避免把管理员自己锁在门外；若服务已在别处被锁定，可在服务器上删除 settings 表中的 ipBlacklist 值后重启，或从 data 目录的备份恢复。</p></section>
    <section v-else-if="logTab === 'notify'" class="card settings-card"><div class="card-heading"><h2><Icon name="BellRing" />通知日志</h2><button class="icon-button" aria-label="刷新通知日志" @click="loadNotifyLogs"><Icon name="RefreshCw" /></button></div><div class="table-scroll"><table><thead><tr><th>通知来源</th><th class="index-column">序号</th><th>发送时间</th><th>标题</th><th>内容</th><th>推送渠道</th><th>发送状态</th></tr></thead><tbody><tr v-for="(item, index) in notifyLogs" :key="item.id"><td>{{ item.source === 'script' ? '脚本通知' : '系统通知' }}<small v-if="item.scriptName" class="secondary-line">{{ item.scriptName }}</small></td><td class="muted">{{ index + 1 }}</td><td class="nowrap">{{ date(item.timestamp) }}</td><td>{{ item.title || '—' }}</td><td class="notify-content">{{ item.content || '—' }}</td><td><code v-if="item.notifyType">{{ item.notifyType }}</code><span v-else class="muted">—</span></td><td><span class="badge" :class="item.status === 0 ? 'success' : 'failed'">{{ item.status === 0 ? '成功' : '失败' }}</span></td></tr><tr v-if="!notifyLogs.length"><td colspan="7" class="muted empty-row">暂无通知记录</td></tr></tbody></table></div><p class="settings-footnote">字段与青龙 GET /api/system/notify-log 一致：timestamp、title、content、status（0 成功、1 失败）、notifyType、source、scriptName。最多保留最近 100 条。</p></section>
    <section v-else-if="logTab === 'system'" class="card settings-card"><div class="card-heading"><h2><Icon name="Terminal" />系统日志</h2><div class="actions"><button class="small-button" @click="loadSystemLog"><Icon name="RefreshCw" :size="15" />刷新</button><button class="small-button danger-text" @click="clearSystemLog"><Icon name="Trash2" :size="15" />清空日志</button></div></div><pre class="system-log-view" ref="systemLogEl">{{ systemLog || '暂无系统日志' }}</pre><p class="settings-footnote">记录面板启动、任务执行与异常等系统事件，默认显示末尾 2MB。任务自身的输出请到「运行日志」查看。</p></section></template>
  </div></div>
  <FormDialog v-if="modal" :title="modal.id ? '编辑应用' : '创建应用'" :fields="[{ key: 'name', label: '应用名称', required: true, placeholder: '例如：MCP 自动化客户端', hint: '名称用于区分应用；保存后生成的 Client ID 用于识别应用，Authorization 用于实际访问。' }, { key: 'expiresAt', label: '有效期至（可选）', type: 'date', hint: '留空表示永久有效；到期后应用无法换取或使用 Authorization。' }]" :initial="modal" :busy="saving" @close="modal = null" @submit="saveApp"><div class="info-banner app-dialog-hint"><Icon name="Info" /><span>Client ID / Secret 用于标识应用并换取访问令牌；MCP / API 实际访问使用单独的 Authorization（Bearer）。创建应用后可在列表中用「创建 Authorization」设置有效期。</span></div><div class="field scope-field"><span>接口权限</span><div class="scope-actions"><button type="button" class="small-button" @click="chosenScopes = [...scopes]">全选</button><button type="button" class="small-button" @click="chosenScopes = []">清空</button></div><div class="scope-options"><label v-for="scope in scopes" :key="scope"><input v-model="chosenScopes" type="checkbox" :value="scope" /><span><code>{{ scope }}</code><small>{{ scopeInfo[scope] }}</small></span></label></div></div></FormDialog>
  <FormDialog v-if="tokenDialog" title="创建 Authorization" :fields="[{ key: 'hours', label: '有效期（小时）', type: 'number', min: 1, max: 720, required: true, hint: 'Authorization 是 Bearer 访问令牌，用于 MCP / API；Client ID 只标识应用，不能直接用于访问。有效期最长 30 天，或勾选永久。' }, { key: 'permanent', label: '永久有效（不自动过期）', type: 'checkbox', hint: '勾选后忽略小时数；应用本身到期时令牌仍会失效。' }]" :initial="{ hours: 24, permanent: false }" :busy="tokenSaving" @close="tokenDialog = null" @submit="createAuthorization"><div class="info-banner"><Icon name="ShieldCheck" /><span>应用：{{ tokenDialog.name }} · Client ID：{{ tokenDialog.client_id }}</span></div></FormDialog>
  <div v-if="updateDialog" class="overlay" @mousedown.self="!updating && (updateDialog = null)"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="update-title"><header><div><span class="eyebrow">版本更新</span><h2 id="update-title">发现新版本 v{{ updateDialog.latest }}</h2></div><button class="icon-button" :disabled="updating" @click="updateDialog = null"><Icon name="X" /></button></header><div class="form-body"><div v-if="!updateDialog.signedUpdates && !updateDialog.allowUnsignedUpdates" class="info-banner danger"><Icon name="CircleAlert" /><span>此构建未嵌入发布公钥，更新已禁用。请先在「关于」页开启允许未签名更新，或改用带签名公钥的正式构建。</span></div><p class="muted">当前 v{{ updateDialog.current }} → 最新 v{{ updateDialog.latest }}。{{ updateDialog.signedUpdates ? '下载后验证发布签名与 SHA-256。' : (updateDialog.allowUnsignedUpdates ? '当前构建未嵌入公钥，仅校验 SHA-256。' : '当前构建未嵌入公钥，已拒绝未签名更新。') }}</p><pre class="release-notes">{{ updateDialog.body || '暂无发行说明' }}</pre><div v-if="progress" class="update-progress"><div class="update-progress-bar"><span :style="{ width: updatePercent(progress) + '%' }" /></div><div class="update-progress-meta"><strong>{{ updateStep || progress.phase }} · {{ updatePercent(progress) }}%</strong><span v-if="progress.total > 0">{{ bytes(progress.received) }} / {{ bytes(progress.total) }}</span><span v-else-if="progress.received">{{ bytes(progress.received) }}</span></div></div><p v-if="progress?.error" class="inline-error" role="alert">{{ progress.error }}</p></div><footer><button class="button" :disabled="updating" @click="updateDialog = null">稍后</button><button class="button primary" :disabled="updating || (!updateDialog.signedUpdates && !updateDialog.allowUnsignedUpdates)" @click="downloadAndApply"><Icon :name="updating ? 'LoaderCircle' : 'Download'" :size="17" :class="{ spin: updating }" />{{ updating ? (updateStep || '更新中…') : '立即更新' }}</button></footer></section></div>
  <Teleport to="body"><div v-if="secret" class="overlay"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="secret-dialog-title"><header><div><span class="eyebrow">应用凭证</span><h2 id="secret-dialog-title">请保存应用凭证</h2></div><button class="icon-button" aria-label="关闭密钥" @click="secret = null"><Icon name="X" /></button></header><div class="form-body"><p class="muted">Client Secret 只显示这一次，关闭后无法再次查看；请立即保存，遗失请点击「重置密钥」。</p><label class="field"><span>Client ID</span><input :value="secret.client_id" readonly /></label><label class="field"><span>Client Secret</span><input :value="secret.client_secret" readonly /></label></div><footer><button class="button" @click="copy(JSON.stringify({ client_id: secret.client_id, client_secret: secret.client_secret }, null, 2))"><Icon name="Copy" />复制凭证</button><button class="button primary" @click="secret = null">已妥善保存</button></footer></section></div><div v-if="authorization" class="overlay"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="authorization-dialog-title"><header><div><span class="eyebrow">Authorization</span><h2 id="authorization-dialog-title">Authorization 已创建</h2></div><button class="icon-button" aria-label="关闭 Authorization" @click="authorization = null"><Icon name="X" /></button></header><div class="form-body"><p class="muted">令牌只显示这一次，关闭后无法再次查看；如需新令牌请重新生成。</p><label class="field"><span>应用 / Client ID</span><input :value="authorization.name + ' · ' + authorization.client_id" readonly /></label><label class="field"><span>Authorization</span><textarea :value="authorization.token" readonly rows="3" /></label><label class="field"><span>有效期至</span><input :value="authorization.expiration ? date(new Date(authorization.expiration * 1000)) : '永久有效'" readonly /></label></div><footer><button class="button" @click="copy(authorization.token)"><Icon name="Copy" />复制 Authorization</button><button class="button primary" @click="authorization = null">已妥善保存</button></footer></section></div><div v-if="restarting" class="overlay"><section class="modal" role="dialog" aria-modal="true" aria-label="正在重启"><div class="form-body"><div class="restart-state"><Icon name="LoaderCircle" class="spin" :size="26" /><div><strong>正在重启服务…</strong><p class="muted">程序已替换，正在等待新版本启动，恢复后页面会自动刷新。</p></div></div></div></section></div></Teleport>
</template>
