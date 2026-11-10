<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { api, action, toast, date, bytes, copy } from '../api'
import { confirmDelete, confirmDialog, detailList } from '../dialogs'
import Icon from '../components/Icon.vue'
import FormDialog from '../components/FormDialog.vue'
import LogPanel from '../components/LogPanel.vue'
import NotificationSettings from '../components/NotificationSettings.vue'
defineProps({ version: String })
const tab = ref('general'), settings = ref({}), runtimes = ref([]), backups = ref([]), apps = ref([]), audit = ref([]), modal = ref(null), secret = ref(null), saving = ref(false), update = ref(null), runId = ref(0), importing = ref(false), account = ref({ username: '', oldPassword: '', password: '' }), mcpToken = ref(''), mcpHours = ref(2), mcpExpiration = ref(0), mcpAppId = ref(0), mcpTokens = ref([]), mcpTokenId = ref(0), tokenDialog = ref(null), tokenSaving = ref(false), authorization = ref(null), viewingSecret = ref(null)
const tabs = [{ id: 'general', label: '常规设置', icon: 'SlidersHorizontal' }, { id: 'notifications', label: '通知', icon: 'Bell' }, { id: 'apps', label: '开放应用 / MCP', icon: 'Plug' }, { id: 'backups', label: '备份恢复', icon: 'DatabaseBackup' }, { id: 'update', label: '版本更新', icon: 'Download' }, { id: 'security', label: '账户安全', icon: 'ShieldCheck' }, { id: 'audit', label: '操作审计', icon: 'ScrollText' }]
const scopes = ['crons', 'envs', 'scripts', 'logs', 'configs', 'subscriptions', 'dependencies', 'mcp']
const scopeInfo = { crons: '任务与调度', envs: '环境变量', scripts: '脚本文件', logs: '运行日志', configs: '系统配置', subscriptions: '订阅同步', dependencies: '依赖安装', mcp: 'MCP 工具' }
const selectedApps = ref([]), selectedBackups = ref([]), batchBusy = ref(false)
const sortedApps = computed(() => [...apps.value].sort((left, right) => new Date(right.createdAt || 0) - new Date(left.createdAt || 0) || right.id - left.id))
const sortedBackups = computed(() => [...backups.value].sort((left, right) => new Date(right.created || 0) - new Date(left.created || 0) || left.name.localeCompare(right.name)))
function appExpired(app) {
  const value = String(app.expiresAt || '').trim()
  if (!value) return false
  const deadline = new Date(value.length <= 10 ? value + 'T23:59:59' : value)
  return !Number.isNaN(deadline.getTime()) && deadline < new Date()
}
const mcpApps = computed(() => sortedApps.value.filter(app => (app.scopes || []).includes('mcp') && !appExpired(app)))
watch(mcpApps, list => { if (!list.some(app => app.id === mcpAppId.value)) mcpAppId.value = list[0]?.id || 0 }, { immediate: true })
watch(mcpAppId, appId => { loadMcpTokens(appId) })
async function loadMcpTokens(appId) {
  mcpTokens.value = []
  mcpTokenId.value = 0
  mcpToken.value = ''
  mcpExpiration.value = 0
  if (!appId) return
  try {
    const list = await api('/apps/' + appId + '/tokens')
    const now = Date.now() / 1000
    mcpTokens.value = (list || []).filter(item => item.permanent || Number(item.expires) > now)
    if (mcpTokens.value.length) selectMcpToken(mcpTokens.value[0].id)
  } catch { /* 页面其他请求会显示错误 */ }
}
function selectMcpToken(id) {
  const numeric = Number(id)
  mcpTokenId.value = numeric
  const item = mcpTokens.value.find(token => token.id === numeric)
  mcpToken.value = item?.token || ''
  mcpExpiration.value = item ? Number(item.expires) : 0
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
  for (const name of original) { try { await api('/backups/' + encodeURIComponent(name), 'DELETE') } catch (error) { failures.push(name); toast(name + '：' + error.message, 'error') } }
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
async function load() { const results = await Promise.allSettled([api('/system/settings'), api('/system/runtimes'), api('/backups'), api('/apps'), api('/audit'), api('/user')]); const targets = [settings, runtimes, backups, apps, audit]; results.forEach((result, index) => { if (result.status === 'fulfilled') { if (index < targets.length) targets[index].value = result.value; else account.value.username = result.value.username } else toast(result.reason.message, 'error') }) }
onMounted(load)
async function save(keys) { saving.value = true; await action(() => api('/system/settings', 'PUT', Object.fromEntries(keys.map(key => [key, settings.value[key]]))), '设置已保存'); saving.value = false }
async function createBackup() { saving.value = true; await action(async () => { await api('/backups/export', 'POST'); backups.value = await api('/backups') }, '备份已创建'); saving.value = false }
async function importBackup(event) { const file = event.target.files?.[0]; event.target.value = ''; if (!file) return; if (!await confirmDialog({ tone: 'danger', icon: 'DatabaseBackup', eyebrow: '恢复备份', title: '导入并覆盖当前数据？', message: '任务、变量、脚本、应用和账号都会被备份中的内容替换。', details: [file.name], note: '系统会先创建恢复前备份；恢复后需要重新登录。', confirmText: '继续导入', cancelText: '取消' })) return; importing.value = true; const form = new FormData(); form.append('file', file); await action(async () => { await api('/backups/import', 'POST', form); window.dispatchEvent(new Event('session-expired')) }, '恢复成功，请使用备份中的账户重新登录'); importing.value = false }
async function deleteBackup(name) { if (!await confirmDelete({ title: '永久删除此备份？', message: '备份 ZIP 会从磁盘移除，之后无法再用它恢复数据。', details: [name], confirmText: '删除备份' })) return; await action(async () => { await api('/backups/' + encodeURIComponent(name), 'DELETE'); backups.value = await api('/backups') }, '备份已删除') }
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
    await loadMcpTokens(app.id)
  }, 'Authorization 已生成')
}
function viewSecret(item) { viewingSecret.value = item }
async function resetFromView() {
  const item = viewingSecret.value
  viewingSecret.value = null
  if (item) await resetSecret(item)
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
async function checkUpdate() { saving.value = true; await action(async () => { update.value = await api('/system/update') }); saving.value = false }
async function downloadUpdate() { await action(async () => { const result = await api('/system/update/download', 'POST'); runId.value = result.runId }, '已开始后台下载与签名验证') }
async function applyUpdate() { if (!await confirmDialog({ tone: 'warning', icon: 'RefreshCw', eyebrow: '应用更新', title: '替换程序文件并重启？', message: '应用后需要重启服务才能使用新版本。', details: [update.value?.latest ? '目标版本 v' + update.value.latest : ''], note: '旧程序会保留为 .previous 文件，可用于手动回滚。', confirmText: '应用更新', cancelText: '取消' })) return; await action(async () => { const result = await api('/system/update/apply', 'POST'); toast(result.message) }) }
async function updateAccount() { saving.value = true; await action(async () => { await api('/user', 'PUT', account.value); window.dispatchEvent(new Event('session-expired')) }, '账户已更新，请重新登录'); saving.value = false }
</script>
<template>
  <div class="settings-layout"><aside class="settings-nav"><button v-for="item in tabs" :key="item.id" :class="{ active: tab === item.id }" @click="changeTab(item.id)"><Icon :name="item.icon" :size="18" />{{ item.label }}</button></aside><div class="settings-content">
    <template v-if="tab === 'general'"><section class="card settings-card"><div class="card-heading"><h2><Icon name="SlidersHorizontal" />执行与存储</h2></div><form @submit.prevent="save(['concurrency', 'timeout', 'retentionDays'])"><div class="settings-fields"><label class="field"><span>全局最大并发数</span><input v-model="settings.concurrency" type="number" min="1" max="32" required /><small>1–32 个后台任务；超出部分自动排队，修改立即生效。</small></label><label class="field"><span>默认执行超时（秒）</span><input v-model="settings.timeout" type="number" min="1" max="604800" required /><small>任务可单独设置超时，超时后清理进程树。</small></label><label class="field"><span>日志保留天数</span><input v-model="settings.retentionDays" type="number" min="1" max="3650" required /><small>每天自动清理过期运行记录和日志。</small></label></div><footer><button type="button" class="button" @click="action(() => api('/system/cleanup', 'PUT'), '已清理过期日志')"><Icon name="Brush" :size="17" />立即清理</button><button class="button primary" :disabled="saving"><Icon name="Check" :size="17" />保存设置</button></footer></form></section><section class="card settings-card"><div class="card-heading"><h2><Icon name="Cpu" />运行环境</h2><span class="badge success">端口 5700</span></div><div class="runtime-list"><div v-for="runtime in runtimes" :key="runtime.name"><span class="resource-icon"><Icon name="Terminal" /></span><div><strong>{{ runtime.name }}</strong><code>{{ runtime.path || '请安装并添加到 PATH' }}</code></div><span class="badge" :class="runtime.available ? 'success' : 'failed'">{{ runtime.available ? '可用' : '未检测到' }}</span></div></div><p class="settings-footnote">Linux 容器镜像包含 Python、Node.js、Bash 和 Git；原生发行包使用宿主运行时。</p></section></template>
    <NotificationSettings v-else-if="tab === 'notifications'" ref="notificationForm" />
    <template v-else-if="tab === 'apps'"><section class="card settings-card"><div class="card-heading"><h2><Icon name="KeyRound" />开放应用</h2><button class="button primary compact" @click="editApp()"><Icon name="Plus" :size="17" />创建应用</button></div><div class="app-list-toolbar"><label class="inline-check"><input type="checkbox" aria-label="选择全部应用" :checked="apps.length > 0 && apps.every(item => selectedApps.includes(item.id))" @change="selectedApps = $event.target.checked ? apps.map(item => item.id) : []" />选择全部</label><span class="muted">{{ sortedApps.length }} 个应用</span></div><div class="app-list"><div v-for="item in sortedApps" :key="item.id" class="app-row" :class="{ 'selected-row': selectedApps.includes(item.id), expired: appExpired(item) }"><input v-model="selectedApps" type="checkbox" :value="item.id" :aria-label="'选择应用 ' + item.name" /><div class="app-main"><div class="app-title"><strong>{{ item.name }}</strong><span v-if="appExpired(item)" class="badge failed"><i />已过期</span><span v-else-if="item.expiresAt" class="badge queued"><i />至 {{ item.expiresAt }}</span><span v-else class="badge"><i />永久</span></div><code class="app-client">Client ID · {{ item.client_id }}</code><div class="labels"><i v-for="scope in item.scopes" :key="scope">{{ scope }}</i></div></div><div class="app-actions"><button class="icon-button" aria-label="编辑权限" title="编辑" @click="editApp(item)"><Icon name="Pencil" :size="16" /></button><button class="icon-button" aria-label="创建 Authorization" title="创建 Authorization" :disabled="appExpired(item)" @click="openAuthorization(item)"><Icon name="ShieldCheck" :size="16" /></button><button class="icon-button" aria-label="查看密钥" title="查看 Client Secret" @click="viewSecret(item)"><Icon name="Eye" :size="16" /></button><button class="icon-button" aria-label="重置应用密钥" title="重置密钥" @click="resetSecret(item)"><Icon name="KeyRound" :size="16" /></button><button class="icon-button danger" aria-label="删除应用" title="删除应用" @click="removeApp(item)"><Icon name="Trash2" :size="16" /></button></div></div><p v-if="!sortedApps.length" class="settings-footnote">创建一个应用，为它分配需要访问的接口权限。Client Secret 可在应用行随时查看。</p></div><div v-if="selectedApps.length" class="bulk-bar"><span>已选 {{ selectedApps.length }} 个应用</span><button class="danger-text" :disabled="batchBusy" @click="deleteSelectedApps">删除所选应用</button><button @click="selectedApps = []">取消选择</button></div><p v-if="!apps.length" class="settings-footnote">创建一个应用，为它分配需要访问的接口权限。Client Secret 可在应用行随时查看。</p></section><section class="card settings-card"><div class="card-heading"><h2><Icon name="Plug" />MCP 服务</h2><span class="badge success"><i />已开放 · 同端口</span></div><div class="settings-fields"><p class="muted">连接地址 <code>{{ location?.origin || '' }}/mcp</code>。使用 Streamable HTTP；应用需要 mcp 权限和对应工具的数据权限。</p><div class="info-banner"><Icon name="Info" /><span>MCP 只需要 Authorization（Bearer）。Client ID / Secret 仅用于标识应用；请选择一个带 mcp 权限且未过期的应用生成令牌。</span></div><form class="token-form" @submit.prevent="generateMcpToken"><label class="field"><span>MCP 应用</span><select v-model="mcpAppId" aria-label="选择 MCP 应用"><option :value="0">请选择应用</option><option v-for="app in mcpApps" :key="app.id" :value="app.id">{{ app.name }}</option></select><small v-if="!mcpApps.length">没有带 mcp 权限且未过期的应用，请先在开放应用中创建。</small></label><label class="field"><span>Authorization 有效期</span><select v-model="mcpHours" aria-label="Authorization 有效期"><option :value="1">1 小时</option><option :value="2">2 小时</option><option :value="6">6 小时</option><option :value="24">24 小时</option><option :value="72">3 天</option><option :value="168">7 天</option><option :value="720">30 天</option><option :value="0">永久</option></select></label><button class="button" :disabled="!mcpAppId">生成 Authorization</button></form><label v-if="mcpTokens.length" class="field"><span>已有 Authorization</span><select v-model="mcpTokenId" aria-label="选择已有 Authorization" @change="selectMcpToken(mcpTokenId)"><option v-for="item in mcpTokens" :key="item.id" :value="item.id">#{{ item.id }} · {{ item.permanent ? '永久' : '至 ' + date(new Date(item.expires * 1000)) }}</option></select></label><div v-if="mcpToken" class="mcp-token-box"><label class="field"><span>当前 Authorization</span><textarea :value="mcpToken" readonly rows="3" aria-label="当前 Authorization" /></label><div class="actions"><button class="button compact" @click="copy(mcpToken)"><Icon name="Copy" :size="15" />复制令牌</button><button class="button danger compact" @click="revokeMcpToken"><Icon name="Trash2" :size="15" />撤销令牌</button></div></div><p v-if="mcpToken" class="mcp-token-expiry">当前 Authorization：{{ mcpExpiration ? '有效期至 ' + date(new Date(mcpExpiration * 1000)) : '永久有效' }}</p><pre class="config-code">{{ mcpConfig }}</pre><button class="button" @click="copy(mcpConfig)"><Icon name="Copy" :size="16" />复制 MCP 配置</button></div></section></template>
    <section v-else-if="tab === 'backups'" class="card settings-card"><div class="card-heading"><h2><Icon name="DatabaseBackup" />备份与恢复</h2></div><div class="settings-fields"><p class="muted">备份包括任务、环境变量、应用、账号、配置、脚本与最近 10000 条运行记录。依赖二进制不随备份迁移，恢复后重新安装即可。</p><div class="info-banner"><Icon name="LockKeyhole" /><span>备份包含敏感变量和账号散列，请妥善保存。为保证一致性，创建和恢复时需要队列清空。</span></div><div class="actions"><button class="button primary" :disabled="saving || importing" @click="createBackup"><Icon name="Download" :size="17" />{{ saving ? '正在备份…' : '创建备份' }}</button><label class="button upload-button" :class="{ disabled: importing }"><Icon name="Upload" :size="17" />{{ importing ? '正在恢复…' : '导入备份' }}<input type="file" accept=".zip" :disabled="importing" @change="importBackup" /></label></div></div><div class="table-scroll"><table><thead><tr><th class="checkbox-column"><input type="checkbox" aria-label="选择全部备份" :checked="backups.length > 0 && backups.every(item => selectedBackups.includes(item.name))" @change="selectedBackups = $event.target.checked ? backups.map(item => item.name) : []" /></th><th>备份文件</th><th>大小</th><th>时间</th><th>操作</th></tr></thead><tbody><tr v-for="item in sortedBackups" :key="item.name" :class="{ 'selected-row': selectedBackups.includes(item.name) }"><td><input v-model="selectedBackups" type="checkbox" :value="item.name" :aria-label="'选择备份 ' + item.name" /></td><td><code>{{ item.name }}</code></td><td>{{ bytes(item.size) }}</td><td>{{ date(item.created) }}</td><td><div class="actions"><a class="icon-button" :href="'/api/backups/' + encodeURIComponent(item.name)" download aria-label="下载备份"><Icon name="Download" :size="17" /></a><button class="icon-button danger" aria-label="删除备份" @click="deleteBackup(item.name)"><Icon name="Trash2" :size="17" /></button></div></td></tr></tbody></table></div><div v-if="selectedBackups.length" class="bulk-bar"><span>已选 {{ selectedBackups.length }} 个备份</span><button class="danger-text" :disabled="batchBusy || importing" @click="deleteSelectedBackups">删除所选备份</button><button @click="selectedBackups = []">取消选择</button></div><p v-if="!backups.length" class="settings-footnote">还没有备份，建议在批量修改或更新前创建一份。</p></section>
    <template v-else-if="tab === 'update'"><section class="card settings-card"><div class="version-hero"><img src="/logo.png" alt="白菜标志" /><div><span class="eyebrow">CABBAGE PANEL</span><h2>大白菜面板 <span>v{{ version }}</span></h2><p>轻盈、开放，为自动化而生。</p></div></div><form @submit.prevent="save(['githubRepo'])"><div class="settings-fields"><label class="field"><span>GitHub Release 仓库</span><input v-model="settings.githubRepo" placeholder="owner/repo" /><small>发布时在构建中嵌入 Ed25519 公钥，下载时验证签名和 SHA-256。</small></label><span class="badge" :class="settings.signedUpdates ? 'success' : 'queued'">{{ settings.signedUpdates ? '已配置发布签名公钥' : '开发构建：尚未嵌入发布公钥' }}</span></div><footer><button class="button" :disabled="saving">保存仓库</button><button type="button" class="button primary" :disabled="saving" @click="checkUpdate"><Icon name="RefreshCw" :size="17" />{{ saving ? '正在检查…' : '检查更新' }}</button></footer></form></section><section v-if="update" class="card settings-card"><div class="card-heading"><h2>{{ update.available ? '发现新版本 ' + update.latest : '当前已是最新稳定版本' }}</h2><span v-if="update.staged" class="badge success">已暂存 {{ update.staged }}</span></div><pre class="release-notes">{{ update.body || '暂无发行说明' }}</pre><footer><button class="button" :disabled="!update.available || !settings.signedUpdates" @click="downloadUpdate">下载并验证</button><button class="button primary" @click="applyUpdate">应用已验证更新</button></footer><p class="settings-footnote">应用后重启服务生效。旧程序保存为 .previous，可手动回滚。容器部署请使用新版本镜像滚动更新。</p></section></template>
    <section v-else-if="tab === 'security'" class="card settings-card"><div class="card-heading"><h2><Icon name="ShieldCheck" />管理员账户</h2></div><form @submit.prevent="updateAccount"><div class="settings-fields"><label class="field"><span>用户名</span><input v-model="account.username" required minlength="2" autocomplete="username" /></label><label class="field"><span>当前密码</span><input v-model="account.oldPassword" type="password" required autocomplete="current-password" /></label><label class="field"><span>新密码</span><input v-model="account.password" type="password" required minlength="12" maxlength="72" autocomplete="new-password" /><small>至少 12 个字符。保存后撤销全部登录和应用 token。</small></label></div><footer><button class="button primary" :disabled="saving">更新账户</button></footer></form></section>
    <section v-else class="card settings-card"><div class="card-heading"><h2><Icon name="ScrollText" />操作审计</h2><button class="icon-button" aria-label="刷新审计" @click="action(async () => audit = await api('/audit'))"><Icon name="RefreshCw" /></button></div><div class="table-scroll"><table><thead><tr><th>操作</th><th>身份</th><th>IP</th><th>时间</th></tr></thead><tbody><tr v-for="item in audit" :key="item.id" :class="{ 'audit-remote': item.remote }"><td><code>{{ item.action }}</code><span v-if="item.remote" class="badge failed audit-badge">异地登录</span></td><td>{{ item.actor }}</td><td><code class="audit-ip" :class="{ remote: item.remote }">{{ item.ip || '—' }}</code></td><td>{{ date(item.created) }}</td></tr></tbody></table></div><p class="settings-footnote">仅显示最近 200 条操作，不记录请求体、密码和变量内容。异地登录表示与上一次管理员登录 IP 不同，使用红色标记提示。</p></section>
  </div></div>
  <FormDialog v-if="modal" :title="modal.id ? '编辑应用' : '创建应用'" :fields="[{ key: 'name', label: '应用名称', required: true, placeholder: '例如：MCP 自动化客户端', hint: '名称用于区分应用；保存后生成的 Client ID 用于识别应用，Authorization 用于实际访问。' }, { key: 'expiresAt', label: '有效期至（可选）', type: 'date', hint: '留空表示永久有效；到期后应用无法换取或使用 Authorization。' }]" :initial="modal" :busy="saving" @close="modal = null" @submit="saveApp"><div class="info-banner app-dialog-hint"><Icon name="Info" /><span>Client ID / Secret 用于标识应用并换取访问令牌；MCP / API 实际访问使用单独的 Authorization（Bearer）。创建应用后可在列表中用「创建 Authorization」设置有效期。</span></div><div class="field scope-field"><span>接口权限</span><div class="scope-actions"><button type="button" class="small-button" @click="chosenScopes = [...scopes]">全选</button><button type="button" class="small-button" @click="chosenScopes = []">清空</button></div><div class="scope-options"><label v-for="scope in scopes" :key="scope"><input v-model="chosenScopes" type="checkbox" :value="scope" /><span><code>{{ scope }}</code><small>{{ scopeInfo[scope] }}</small></span></label></div></div></FormDialog>
  <FormDialog v-if="tokenDialog" title="创建 Authorization" :fields="[{ key: 'hours', label: '有效期（小时）', type: 'number', min: 1, max: 720, required: true, hint: 'Authorization 是 Bearer 访问令牌，用于 MCP / API；Client ID 只标识应用，不能直接用于访问。有效期最长 30 天，或勾选永久。' }, { key: 'permanent', label: '永久有效（不自动过期）', type: 'checkbox', hint: '勾选后忽略小时数；应用本身到期时令牌仍会失效。' }]" :initial="{ hours: 24, permanent: false }" :busy="tokenSaving" @close="tokenDialog = null" @submit="createAuthorization"><div class="info-banner"><Icon name="ShieldCheck" /><span>应用：{{ tokenDialog.name }} · Client ID：{{ tokenDialog.client_id }}</span></div></FormDialog>
  <Teleport to="body"><div v-if="secret" class="overlay"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="secret-dialog-title"><header><div><span class="eyebrow">应用凭证</span><h2 id="secret-dialog-title">请保存应用凭证</h2></div><button class="icon-button" aria-label="关闭密钥" @click="secret = null"><Icon name="X" /></button></header><div class="form-body"><p class="muted">Client Secret 会保存在本地数据库，可在应用行点击「查看密钥」再次查看；请妥善保护 data/ 和备份。</p><label class="field"><span>Client ID</span><input :value="secret.client_id" readonly /></label><label class="field"><span>Client Secret</span><input :value="secret.client_secret" readonly /></label></div><footer><button class="button" @click="copy(JSON.stringify({ client_id: secret.client_id, client_secret: secret.client_secret }, null, 2))"><Icon name="Copy" />复制凭证</button><button class="button primary" @click="secret = null">已妥善保存</button></footer></section></div><div v-if="viewingSecret" class="overlay"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="view-secret-dialog-title"><header><div><span class="eyebrow">应用凭证</span><h2 id="view-secret-dialog-title">Client Secret</h2></div><button class="icon-button" aria-label="关闭密钥查看" @click="viewingSecret = null"><Icon name="X" /></button></header><div class="form-body"><p class="muted">Client Secret 以明文保存在本地数据库中，仅管理员可见；请妥善保护 data/ 和备份。</p><label class="field"><span>应用</span><input :value="viewingSecret.name" readonly /></label><label class="field"><span>Client ID</span><input :value="viewingSecret.client_id" readonly /></label><label class="field"><span>Client Secret</span><textarea :value="viewingSecret.client_secret || '旧版本应用未保存明文密钥，请重置一次生成新密钥。'" readonly rows="2" aria-label="当前 Client Secret" /></label></div><footer><button v-if="viewingSecret.client_secret" class="button" @click="copy(viewingSecret.client_secret)"><Icon name="Copy" />复制 Secret</button><button v-else class="button danger" @click="resetFromView">重置并保存新密钥</button><button class="button primary" @click="viewingSecret = null">关闭</button></footer></section></div><div v-if="authorization" class="overlay"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="authorization-dialog-title"><header><div><span class="eyebrow">Authorization</span><h2 id="authorization-dialog-title">Authorization 已创建</h2></div><button class="icon-button" aria-label="关闭 Authorization" @click="authorization = null"><Icon name="X" /></button></header><div class="form-body"><p class="muted">令牌已登记在应用中，可在 MCP 区域的选择框中再次查看和复制；请妥善保存。</p><label class="field"><span>应用 / Client ID</span><input :value="authorization.name + ' · ' + authorization.client_id" readonly /></label><label class="field"><span>Authorization</span><textarea :value="authorization.token" readonly rows="3" /></label><label class="field"><span>有效期至</span><input :value="authorization.expiration ? date(new Date(authorization.expiration * 1000)) : '永久有效'" readonly /></label></div><footer><button class="button" @click="copy(authorization.token)"><Icon name="Copy" />复制 Authorization</button><button class="button primary" @click="authorization = null">已妥善保存</button></footer></section></div><LogPanel v-if="runId" :key="runId" :run-id="runId" title="版本下载与验证" @close="runId = 0; checkUpdate()" /></Teleport>
</template>
