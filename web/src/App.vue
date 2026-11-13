<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { api, action, notices } from './api'
import { checkLeaveGuards } from './guard'
import Icon from './components/Icon.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import Dashboard from './pages/Dashboard.vue'
import Tasks from './pages/Tasks.vue'
import Resources from './pages/Resources.vue'
import Scripts from './pages/Scripts.vue'
import Logs from './pages/Logs.vue'
import ApiDocs from './pages/ApiDocs.vue'
import Settings from './pages/Settings.vue'

const ready = ref(false), loggedIn = ref(false), initialized = ref(true), busy = ref(false), error = ref('')
const user = ref(''), version = ref(''), menuOpen = ref(false), globalSearch = ref(''), updateInfo = ref(null), settingsTab = ref('')
// 公告弹层：异步加载，失败静默跳过，不影响主界面。
const noticeOpen = ref(false), noticeText = ref(''), noticeLoading = ref(false), noticeLoaded = ref(false)
async function openNotice() {
  noticeOpen.value = true
  if (noticeLoaded.value || noticeLoading.value) return
  noticeLoading.value = true
  try {
    const result = await api('/system/notice')
    noticeText.value = (result && result.text) || ''
    noticeLoaded.value = true
  } catch {
    // 网络不可达时保持空文本，不弹出错误提示。
    noticeText.value = ''
  } finally {
    noticeLoading.value = false
  }
}
const login = ref({ username: '', password: '', setupToken: '' })
const remember = ref(false)
const rememberKey = 'cabbage-remember-login'
// 记住的凭据只做 Base64 编码，避免在 localStorage 里以明文直接可见。
// 这是本地单用户面板的便利功能，不构成安全边界；共用电脑请勿勾选。
function encodeRemember(value) {
  try {
    const bytes = new TextEncoder().encode(JSON.stringify(value))
    let binary = ''
    bytes.forEach(byte => { binary += String.fromCharCode(byte) })
    return btoa(binary)
  } catch { return '' }
}
function decodeRemember(text) {
  try {
    const binary = atob(text)
    const bytes = Uint8Array.from(binary, char => char.charCodeAt(0))
    return JSON.parse(new TextDecoder().decode(bytes))
  } catch { return null }
}
function restoreRemember() {
  if (typeof localStorage === 'undefined') return
  const saved = decodeRemember(localStorage.getItem(rememberKey) || '')
  if (saved && saved.username) {
    login.value.username = saved.username
    login.value.password = saved.password || ''
    remember.value = true
  }
}
function persistRemember() {
  if (typeof localStorage === 'undefined') return
  if (remember.value && login.value.username) {
    localStorage.setItem(rememberKey, encodeRemember({ username: login.value.username, password: login.value.password }))
  } else {
    localStorage.removeItem(rememberKey)
  }
}
const navigation = [
  { id: 'dashboard', label: '仪表盘', icon: 'LayoutDashboard' },
  { id: 'tasks', label: '定时任务', icon: 'Clock3' },
  { id: 'scripts', label: '脚本管理', icon: 'SquareTerminal' },
  { id: 'envs', label: '环境变量', icon: 'KeyRound' },
  { id: 'logs', label: '运行日志', icon: 'FileText' },
  { id: 'subscriptions', label: '订阅管理', icon: 'GitBranch' },
  { id: 'dependencies', label: '依赖管理', icon: 'Package' },
  { id: 'api-docs', label: '接口文档', icon: 'BookOpen' },
  { id: 'settings', label: '系统设置', icon: 'Settings2' },
]
const routeAliases = { configs: 'scripts' }
function normalizeRoute(value) {
  const id = routeAliases[value] || value
  return navigation.some(item => item.id === id) ? id : 'dashboard'
}
const page = ref(normalizeRoute(location.hash.slice(1).split('?')[0] || 'dashboard'))
const current = computed(() => navigation.find(item => item.id === page.value) || navigation[0])
let approvedHash = ''
let navigating = false
function applyPage(id) {
  page.value = id
}
watch([loggedIn, page], () => {
  document.title = loggedIn.value ? current.value.label + ' · 大白菜面板' : '大白菜面板'
}, { immediate: true })
async function navigate(id) {
  menuOpen.value = false
  if (id === page.value || navigating) return
  navigating = true
  try {
    if (!await checkLeaveGuards()) return
    const target = normalizeRoute(id)
    if (location.hash.slice(1).split('?')[0] === target) { applyPage(target); return }
    approvedHash = target
    location.hash = target
  } finally {
    navigating = false
  }
}
async function onHash() {
  const raw = location.hash.slice(1).split('?')[0]
  const next = normalizeRoute(raw)
  if (next === page.value) {
    approvedHash = ''
    if (raw !== next) history.replaceState(null, '', '#' + next)
    return
  }
  if (approvedHash === next) { approvedHash = ''; applyPage(next); return }
  if (navigating) return
  navigating = true
  try {
    if (!await checkLeaveGuards()) { history.replaceState(null, '', '#' + page.value); return }
    if (raw !== next) history.replaceState(null, '', '#' + next)
    applyPage(next)
  } finally {
    navigating = false
  }
}
function expired() { loggedIn.value = false; login.value.password = ''; error.value = '登录已过期，请重新登录' }
let updateTimer = null
async function checkUpdateHint() { if (!loggedIn.value) return; try { updateInfo.value = await api('/system/update') } catch { /* 未配置仓库或离线时静默忽略 */ } }
async function openUpdate() { settingsTab.value = 'update'; await navigate('settings') }
watch(loggedIn, value => { if (value) checkUpdateHint() })
function keyboard(event) { if ((event.ctrlKey || event.metaKey) && event.key === 'k') { event.preventDefault(); document.querySelector('#global-search')?.focus() } }
async function bootstrap() {
  try {
    const state = await api('/system/init'); initialized.value = state.isInitialized; version.value = state.version
    if (state.isInitialized) { try { const result = await api('/user'); user.value = result.username; loggedIn.value = true; error.value = '' } catch { error.value = '' } }
  } catch (failure) { error.value = failure.message } finally { ready.value = true }
}
async function submitLogin() {
  busy.value = true; error.value = ''
  try {
    if (!initialized.value) { await api('/user/init', 'POST', login.value); initialized.value = true }
    const result = await api('/user/login', 'POST', login.value); user.value = result.username; loggedIn.value = true
    persistRemember()
    login.value.password = ''; login.value.setupToken = ''
  } catch (failure) { error.value = failure.message } finally { busy.value = false }
}
async function logout() { await action(() => api('/user/logout', 'POST')); loggedIn.value = false }
onMounted(async () => {
  // 挂载后再恢复，确保 v-model 已完成首次绑定，密码框能正确回填。
  restoreRemember()
  await nextTick()
  await bootstrap()
  if (loggedIn.value) checkUpdateHint()
  updateTimer = setInterval(() => { if (loggedIn.value) checkUpdateHint() }, 30 * 60 * 1000)
  window.addEventListener('hashchange', onHash); window.addEventListener('session-expired', expired); window.addEventListener('keydown', keyboard)
})
onUnmounted(() => { if (updateTimer) clearInterval(updateTimer); window.removeEventListener('hashchange', onHash); window.removeEventListener('session-expired', expired); window.removeEventListener('keydown', keyboard) })
</script>

<template>
  <div v-if="!ready" class="loading-screen"><img src="/logo.png" alt="" /><span>正在唤醒您的工作空间…</span></div>
  <div v-else-if="!loggedIn" class="login-screen">
    <div class="login-brand"><img src="/logo.png" alt="白菜标志" /><span>大白菜面板</span><small>A LITTLE GREEN. A LOT DONE.</small></div>
    <section class="login-card"><span class="eyebrow">YOUR AUTOMATION WORKSPACE</span><h1>{{ initialized ? '欢迎回来。' : '种下自动化的第一步。' }}</h1><p>{{ initialized ? '登录大白菜面板，让工作轻一点。' : '创建管理员账户，开始您的自动化之旅。' }}</p>
      <form @submit.prevent="submitLogin"><label v-if="!initialized" class="field"><span>首次设置密钥</span><input v-model="login.setupToken" aria-label="首次设置密钥" required type="password" autocomplete="off" placeholder="查看启动日志或 data/setup.key" /><small>仅首次初始化需要，用于确认您拥有这台服务器。</small></label><label class="field"><span>用户名</span><input v-model="login.username" autocomplete="username" required minlength="2" placeholder="请输入用户名" /></label><label class="field"><span>密码</span><input v-model="login.password" type="password" :autocomplete="initialized ? 'current-password' : 'new-password'" required :minlength="initialized ? 1 : 6" :placeholder="initialized ? '请输入密码' : '至少 6 个字符'" /></label><label v-if="initialized" class="login-remember"><input v-model="remember" type="checkbox" /><span>记住账号密码</span><small>仅保存在本机浏览器；共用电脑请勿勾选。</small></label><p v-if="error" class="inline-error" role="alert">{{ error }}</p><button class="button primary login-button" :disabled="busy"><Icon :name="busy ? 'LoaderCircle' : 'ArrowRight'" :class="{ spin: busy }" />{{ busy ? '请稍候…' : initialized ? '进入工作空间' : '创建并登录' }}</button></form>
      <div class="login-note"><Icon name="ShieldCheck" :size="15" />本地数据 · 自主掌控 · 轻量运行</div>
    </section><div class="login-footer">大白菜面板 v{{ version }} · 把时间留给更重要的事</div>
  </div>
  <div v-else class="app-shell">
    <div v-if="menuOpen" class="sidebar-scrim" @click="menuOpen = false" />
    <aside class="sidebar" :class="{ open: menuOpen }"><a class="brand" href="#dashboard" @click="navigate('dashboard')"><img src="/logo.png" alt="" /><div>大白菜面板<small>CABBAGE PANEL</small></div></a>
      <div class="nav-caption">工作空间</div><nav aria-label="主导航"><button v-for="item in navigation" :key="item.id" :class="{ active: page === item.id }" :aria-current="page === item.id ? 'page' : undefined" @click="navigate(item.id)"><Icon :name="item.icon" /><span>{{ item.label }}</span><span v-if="page === item.id" class="nav-dot" /></button></nav>
      <div class="sidebar-bottom"><button v-if="updateInfo && updateInfo.available" type="button" class="sidebar-status sidebar-status-update" @click="openUpdate" :title="'发现新版本 v' + updateInfo.latest + '，点击更新'"><span class="update-dot" /><div><strong>发现新版本 v{{ updateInfo.latest }}</strong><small>当前 v{{ version }} · 点击更新</small></div></button><div v-else class="sidebar-status"><span class="live-dot" /><div><strong>服务已连接</strong><small>v{{ version }} · 本地数据</small></div></div></div>
    </aside>
    <div class="main-shell"><header class="topbar"><button class="icon-button mobile-menu" aria-label="打开导航" @click="menuOpen = !menuOpen"><Icon name="Menu" /></button><form class="global-search" @submit.prevent="navigate('tasks')"><Icon name="Search" /><input id="global-search" v-model="globalSearch" placeholder="搜索任务、命令、关键词…" aria-label="搜索任务" /><kbd>Ctrl K</kbd></form><div class="topbar-right"><span class="topbar-date">{{ new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }) }}</span><button class="icon-button" title="查看通知" aria-label="查看通知" @click="openNotice"><Icon name="Bell" /></button><div class="user-avatar">{{ user.slice(0, 1).toUpperCase() }}</div><span class="user-name">{{ user }}</span><button class="icon-button" title="退出登录" aria-label="退出登录" @click="logout"><Icon name="LogOut" :size="17" /></button></div></header>
      <main>
        <Dashboard v-if="page === 'dashboard'" @navigate="navigate" />
        <Tasks v-else-if="page === 'tasks'" :global-search="globalSearch" />
        <Scripts v-else-if="page === 'scripts'" />
        <Logs v-else-if="page === 'logs'" />
        <Resources v-else-if="['envs', 'subscriptions', 'dependencies'].includes(page)" :key="page" :kind="page" />
        <ApiDocs v-else-if="page === 'api-docs'" @navigate="navigate" />
        <Settings v-else-if="page === 'settings'" :version="version" :focus-tab="settingsTab" @consume-tab="settingsTab = ''" />
      </main><footer class="page-footer"><span>大白菜面板 <span class="footer-dot">·</span> 让自动化回归简单</span><span>Go + Vue · SQLite</span></footer>
    </div>
  </div>
  <div v-if="noticeOpen" class="overlay notice-overlay" @mousedown.self="noticeOpen = false"><section class="modal notice-modal" role="dialog" aria-modal="true" aria-labelledby="notice-title"><header><div><span class="eyebrow">NOTICE</span><h2 id="notice-title">通知</h2></div><button class="icon-button" aria-label="关闭通知" @click="noticeOpen = false"><Icon name="X" /></button></header><div class="notice-body"><p v-if="noticeLoading" class="muted">正在获取通知…</p><p v-else-if="noticeText" class="notice-text">{{ noticeText }}</p><p v-else class="muted">暂无通知内容。</p></div><footer><a class="small-button" href="https://github.com/bucai2417/cabbage-panel" target="_blank" rel="noopener noreferrer"><Icon name="GitBranch" :size="15" />项目主页</a><button class="button" @click="noticeOpen = false">关闭</button></footer></section></div>
  <div class="toast-stack" aria-live="polite"><div v-for="notice in notices" :key="notice.id" class="toast" :class="notice.type"><Icon :name="notice.type === 'error' ? 'CircleAlert' : 'CircleCheck'" /><span>{{ notice.message }}</span><button class="icon-button" aria-label="关闭提示" @click="notices = notices.filter(item => item.id !== notice.id)"><Icon name="X" :size="14" /></button></div></div>
  <ConfirmDialog />
</template>
