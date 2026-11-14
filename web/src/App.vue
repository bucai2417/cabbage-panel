<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { api, action, notices, toast } from './api'
import { checkLeaveGuards } from './guard'
import { clearRememberedPassword, loadRememberedPassword, saveRememberedPassword } from './credential-store'
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
const user = ref(''), version = ref(''), menuOpen = ref(false), globalSearch = ref(''), updateInfo = ref(null), settingsTab = ref(''), runningTasks = ref(0)
// 公告弹层：异步加载，失败静默跳过，不影响主界面。
const noticeOpen = ref(false), noticeText = ref(''), noticeLoading = ref(false), noticeLoaded = ref(false)
// 未读红点：对比公告内容的哈希与上次已读，避免同一公告重复提示。
const noticeUnread = ref(false), noticeReadKey = 'cabbage-notice-read'
function noticeHash(text) { let h = 2166136261; for (let i = 0; i < text.length; i++) { h ^= text.charCodeAt(i); h = Math.imul(h, 16777619) } return (h >>> 0).toString(16) + '-' + text.length }
function markNoticeRead() { if (noticeText.value) { try { localStorage.setItem(noticeReadKey, noticeHash(noticeText.value)) } catch { /* 忽略存储失败 */ } } noticeUnread.value = false }
async function checkNotice() {
  try {
    const result = await api('/system/notice')
    noticeText.value = (result && result.text) || ''
    noticeLoaded.value = true
    let saved = ''
    try { saved = localStorage.getItem(noticeReadKey) || '' } catch { /* 忽略存储失败 */ }
    noticeUnread.value = !!noticeText.value && noticeHash(noticeText.value) !== saved
  } catch { /* 离线或未配置时静默 */ }
}
async function openNotice() {
  noticeOpen.value = true
  if (noticeLoading.value) return
  noticeLoading.value = true
  try {
    if (!noticeLoaded.value) {
      const result = await api('/system/notice')
      noticeText.value = (result && result.text) || ''
      noticeLoaded.value = true
    }
  } catch {
    // 网络不可达时保持空文本，不弹出错误提示。
    noticeText.value = ''
  } finally {
    noticeLoading.value = false
  }
  markNoticeRead()
}
const login = ref({ username: '', password: '', setupToken: '' })
const remember = ref(false)
const rememberPassword = ref(false)
const rememberKey = 'cabbage-remember-user'
const legacyRememberKey = 'cabbage-remember-login'
// 只在本地记住用户名；密码绝不写入浏览器存储，保持登录依赖服务端 HttpOnly 会话 Cookie。
async function restoreRemember() {
  if (typeof localStorage === 'undefined') return
  try {
    // 旧版本会把用户名和密码一起 Base64 存储，这里迁移到新的本机凭据存储。
    const legacy = localStorage.getItem(legacyRememberKey)
    if (legacy) {
      let saved = null
      try { saved = JSON.parse(new TextDecoder().decode(Uint8Array.from(atob(legacy), char => char.charCodeAt(0)))) } catch { /* 忽略损坏数据 */ }
      localStorage.removeItem(legacyRememberKey)
      if (saved && saved.username) localStorage.setItem(rememberKey, saved.username)
      if (saved && saved.password && await saveRememberedPassword(saved.password)) {
        login.value.password = saved.password
        rememberPassword.value = true
      }
    }
    const username = localStorage.getItem(rememberKey) || ''
    if (username) { login.value.username = username; remember.value = true }
    if (!login.value.password) {
      const password = await loadRememberedPassword()
      if (password) { login.value.password = password; rememberPassword.value = true }
    }
  } catch { /* 忽略存储失败 */ }
}
function persistRemember(username) {
  if (typeof localStorage === 'undefined') return
  try {
    if ((remember.value || rememberPassword.value) && username) localStorage.setItem(rememberKey, username)
    else localStorage.removeItem(rememberKey)
  } catch { /* 忽略存储失败 */ }
}
function onRememberPasswordChange() {
  if (!rememberPassword.value) clearRememberedPassword()
}
const navigation = [
  { id: 'dashboard', label: '仪表盘', icon: 'LayoutDashboard' },
  { id: 'tasks', label: '定时任务', icon: 'Clock3' },
  { id: 'envs', label: '环境变量', icon: 'KeyRound' },
  { id: 'scripts', label: '脚本管理', icon: 'SquareTerminal' },
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
const openTabKey = 'cabbage-open-tabs'
function readOpenTabs() {
  try {
    const saved = JSON.parse(localStorage.getItem(openTabKey) || '[]')
    const ids = (Array.isArray(saved) ? saved : []).map(id => routeAliases[id] || id).filter((id, index, list) => navigation.some(item => item.id === id) && list.indexOf(id) === index)
    return ['dashboard', ...ids.filter(id => id !== 'dashboard')]
  } catch { return ['dashboard'] }
}
const initialTabs = readOpenTabs()
if (!initialTabs.includes(page.value)) initialTabs.push(page.value)
const openTabIds = ref(initialTabs)
const openTabs = computed(() => openTabIds.value.map(id => navigation.find(item => item.id === id)).filter(Boolean))
const tabsTrack = ref(null), tabMenuOpen = ref(false)
const pageScroll = new Map()
function persistOpenTabs() { try { localStorage.setItem(openTabKey, JSON.stringify(openTabIds.value)) } catch { /* 忽略存储失败 */ } }
function addOpenTab(id) {
  const target = normalizeRoute(id)
  if (!openTabIds.value.includes(target)) {
    openTabIds.value = [...openTabIds.value, target]
    persistOpenTabs()
  }
}
function scrollActiveTab() {
  nextTick(() => {
    const track = tabsTrack.value, active = track?.querySelector('.page-tab.active')
    if (!track || !active) return
    const left = active.offsetLeft, right = left + active.offsetWidth
    if (left < track.scrollLeft) track.scrollTo({ left: Math.max(0, left - 12), behavior: 'smooth' })
    else if (right > track.scrollLeft + track.clientWidth) track.scrollTo({ left: right - track.clientWidth + 12, behavior: 'smooth' })
  })
}
watch(page, id => { addOpenTab(id); scrollActiveTab() }, { immediate: true })
async function closeTab(id) {
  if (id === 'dashboard') return
  const index = openTabIds.value.indexOf(id)
  if (index < 0) return
  if (!await checkLeaveGuards()) return
  if (page.value === id) {
    const next = openTabIds.value[index - 1] || openTabIds.value[index + 1] || 'dashboard'
    if (!await navigate(next)) return
  }
  openTabIds.value = openTabIds.value.filter(item => item !== id)
  pageScroll.delete(id)
  persistOpenTabs()
}
async function closeOtherTabs() {
  tabMenuOpen.value = false
  if (!await checkLeaveGuards()) return
  const keep = page.value === 'dashboard' ? ['dashboard'] : ['dashboard', page.value]
  for (const id of openTabIds.value) if (!keep.includes(id)) pageScroll.delete(id)
  openTabIds.value = keep
  persistOpenTabs()
}
async function closeAllTabs() {
  tabMenuOpen.value = false
  if (!await checkLeaveGuards()) return
  if (page.value !== 'dashboard' && !await navigate('dashboard')) return
  for (const id of openTabIds.value) if (id !== 'dashboard') pageScroll.delete(id)
  openTabIds.value = ['dashboard']
  persistOpenTabs()
}
function closeTabMenu(event) { if (!event.target.closest('.page-tabs-actions')) tabMenuOpen.value = false }
let approvedHash = ''
let navigating = false
function applyPage(id) {
  if (id === page.value) return
  pageScroll.set(page.value, window.scrollY)
  addOpenTab(id)
  page.value = id
  nextTick(() => window.scrollTo({ top: pageScroll.get(id) || 0, behavior: 'auto' }))
}
watch([loggedIn, page], () => {
  document.title = loggedIn.value ? current.value.label + ' · 大白菜面板' : '大白菜面板'
}, { immediate: true })
async function navigate(id) {
  menuOpen.value = false
  const target = normalizeRoute(id)
  if (target === page.value || navigating) return false
  navigating = true
  try {
    approvedHash = target
    if (location.hash.slice(1).split('?')[0] !== target) {
      location.hash = target
    } else if (location.hash !== '#' + target) {
      history.replaceState(null, '', '#' + target)
    }
    applyPage(target)
    return true
  } finally {
    navigating = false
  }
}
function onHash() {
  const raw = location.hash.slice(1).split('?')[0]
  const next = normalizeRoute(raw)
  if (next === page.value) {
    approvedHash = ''
    if (raw !== next) history.replaceState(null, '', '#' + next)
    return
  }
  if (approvedHash === next) { approvedHash = ''; applyPage(next); return }
  if (navigating) return
  if (raw !== next) history.replaceState(null, '', '#' + next)
  applyPage(next)
}
function expired() { loggedIn.value = false; login.value.password = ''; error.value = '登录已过期，请重新登录' }
let updateTimer = null
let activityTimer = null
async function checkUpdateHint() { if (!loggedIn.value) return; try { const info = await api('/system/update'); if (info && typeof info.latest === 'string') info.latest = info.latest.replace(/^v/i, ''); updateInfo.value = info } catch { /* 未配置仓库或离线时静默忽略 */ } }
async function refreshActivity() {
  if (!loggedIn.value || document.hidden) return
  try {
    const activity = await api('/system/activity')
    runningTasks.value = Math.max(0, Number(activity.running) || 0)
  } catch { /* 静默忽略网络抖动 */ }
}
function stopActivityPolling() {
  if (activityTimer) clearInterval(activityTimer)
  activityTimer = null
  runningTasks.value = 0
}
function startActivityPolling() {
  stopActivityPolling()
  refreshActivity()
  activityTimer = setInterval(refreshActivity, 5000)
}
function onVisibilityChange() { if (!document.hidden) refreshActivity() }
async function openUpdate() { settingsTab.value = 'about'; await navigate('settings') }
watch(loggedIn, value => { if (value) { checkUpdateHint(); checkNotice(); startActivityPolling() } else stopActivityPolling() })
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
    const password = login.value.password
    const keep = initialized.value && remember.value
    if (!initialized.value) { await api('/user/init', 'POST', login.value); initialized.value = true }
    const result = await api('/user/login', 'POST', { username: login.value.username, password, remember: keep })
    user.value = result.username; loggedIn.value = true
    if (rememberPassword.value) {
      if (!await saveRememberedPassword(password)) {
        rememberPassword.value = false
        clearRememberedPassword()
        toast('无法在本机保存密码，请检查浏览器存储权限', 'error')
      }
    } else {
      clearRememberedPassword()
    }
    persistRemember(result.username || login.value.username)
    login.value.password = ''; login.value.setupToken = ''
  } catch (failure) { error.value = failure.message } finally { busy.value = false }
}
async function logout() { await action(() => api('/user/logout', 'POST')); loggedIn.value = false }
onMounted(async () => {
  // 挂载后再恢复，确保 v-model 已完成首次绑定，密码框能正确回填。
  await restoreRemember()
  await nextTick()
  await bootstrap()
  if (loggedIn.value) { checkUpdateHint(); checkNotice(); startActivityPolling() }
  updateTimer = setInterval(() => { if (loggedIn.value) checkUpdateHint() }, 30 * 60 * 1000)
  window.addEventListener('hashchange', onHash); window.addEventListener('session-expired', expired); window.addEventListener('keydown', keyboard); document.addEventListener('pointerdown', closeTabMenu); document.addEventListener('visibilitychange', onVisibilityChange)
})
onUnmounted(() => { if (updateTimer) clearInterval(updateTimer); if (activityTimer) clearInterval(activityTimer); window.removeEventListener('hashchange', onHash); window.removeEventListener('session-expired', expired); window.removeEventListener('keydown', keyboard); document.removeEventListener('pointerdown', closeTabMenu); document.removeEventListener('visibilitychange', onVisibilityChange) })
</script>

<template>
  <div v-if="!ready" class="loading-screen"><img src="/logo.png" alt="" /><span>正在唤醒您的工作空间…</span></div>
  <div v-else-if="!loggedIn" class="login-screen">
    <div class="login-brand"><img src="/logo.png" alt="白菜标志" /><span>大白菜面板</span><small>A LITTLE GREEN. A LOT DONE.</small></div>
    <section class="login-card"><h1>{{ initialized ? '欢迎回来。' : '种下自动化的第一步。' }}</h1><p>{{ initialized ? '登录大白菜面板，让工作轻一点。' : '创建管理员账户，开始您的自动化之旅。' }}</p>
      <form @submit.prevent="submitLogin"><label v-if="!initialized" class="field"><span>首次设置密钥</span><input v-model="login.setupToken" aria-label="首次设置密钥" required type="password" autocomplete="off" placeholder="打开 data/setup.key 查看" /><small>仅首次初始化需要，用于确认您拥有这台服务器。</small></label><label class="field"><span>用户名</span><input v-model="login.username" autocomplete="username" required minlength="2" placeholder="请输入用户名" /></label><label class="field"><span>密码</span><input v-model="login.password" type="password" :autocomplete="initialized ? 'current-password' : 'new-password'" required :minlength="initialized ? 1 : 6" :placeholder="initialized ? '请输入密码' : '至少 6 个字符'" /></label><div v-if="initialized" class="login-remember-row"><label class="login-remember"><input v-model="remember" type="checkbox" /><span>保持登录 30 天</span></label><label class="login-remember"><input v-model="rememberPassword" type="checkbox" @change="onRememberPasswordChange" /><span>记住密码</span></label></div><p v-if="error" class="inline-error" role="alert">{{ error }}</p><button class="button primary login-button" :disabled="busy"><Icon :name="busy ? 'LoaderCircle' : 'ArrowRight'" :class="{ spin: busy }" />{{ busy ? '请稍候…' : initialized ? '进入工作空间' : '创建并登录' }}</button></form>
      <div class="login-note"><Icon name="ShieldCheck" :size="15" />本地数据 · 自主掌控 · 轻量运行</div>
    </section><div class="login-footer">大白菜面板 v{{ version }} · 把时间留给更重要的事</div>
  </div>
  <div v-else class="app-shell">
    <div v-if="menuOpen" class="sidebar-scrim" @click="menuOpen = false" />
    <aside class="sidebar" :class="{ open: menuOpen }"><a class="brand" href="#dashboard" @click="navigate('dashboard')"><img src="/logo.png" alt="" /><div>大白菜面板<small>CABBAGE PANEL</small></div></a>
      <div class="nav-caption">工作空间</div><nav aria-label="主导航"><button v-for="item in navigation" :key="item.id" :class="{ active: page === item.id }" :aria-current="page === item.id ? 'page' : undefined" @click="navigate(item.id)"><Icon :name="item.icon" /><span>{{ item.label }}</span><span v-if="(item.id === 'tasks' && runningTasks > 0) || page === item.id" class="nav-trailing"><span v-if="item.id === 'tasks' && runningTasks > 0" class="nav-badge" :aria-label="runningTasks + ' 个任务运行中'">{{ runningTasks > 99 ? '99+' : runningTasks }}</span><span v-if="page === item.id" class="nav-dot" /></span></button></nav>
      <div class="sidebar-bottom"><button v-if="updateInfo && updateInfo.available" type="button" class="sidebar-status sidebar-status-update" @click="openUpdate" :title="'发现新版本 v' + updateInfo.latest + '，点击更新'"><span class="update-dot" /><div><strong>发现新版本 v{{ updateInfo.latest }}</strong><small>当前 v{{ version }} · 点击更新</small></div></button><div v-else class="sidebar-status"><span class="live-dot" /><div><strong>服务已连接</strong><small>v{{ version }} · 本地数据</small></div></div></div>
    </aside>
    <div class="main-shell"><header class="topbar"><button class="icon-button mobile-menu" aria-label="打开导航" @click="menuOpen = !menuOpen"><Icon name="Menu" /></button><form class="global-search" @submit.prevent="navigate('tasks')"><Icon name="Search" /><input id="global-search" v-model="globalSearch" placeholder="搜索任务、命令、关键词…" aria-label="搜索任务" /><kbd>Ctrl K</kbd></form><div class="topbar-right"><span class="topbar-date">{{ new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }) }}</span><button class="icon-button notice-bell" title="查看通知" aria-label="查看通知" @click="openNotice"><Icon name="Bell" /><span v-if="noticeUnread" class="notice-dot" aria-hidden="true" /></button><div class="user-avatar">{{ user.slice(0, 1).toUpperCase() }}</div><span class="user-name">{{ user }}</span><button class="icon-button" title="退出登录" aria-label="退出登录" @click="logout"><Icon name="LogOut" :size="17" /></button></div></header>
      <nav class="page-tabs" aria-label="已打开页面">
        <div ref="tabsTrack" class="page-tabs-track" role="tablist">
          <div v-for="tab in openTabs" :key="tab.id" class="page-tab" :class="{ active: page === tab.id }" role="presentation">
            <button type="button" class="page-tab-main" role="tab" :aria-selected="page === tab.id" @click="navigate(tab.id)">
              <Icon :name="tab.icon" :size="14" />
              <span>{{ tab.label }}</span>
            </button>
            <button v-if="tab.id !== 'dashboard'" type="button" class="page-tab-close" :aria-label="'关闭 ' + tab.label" title="关闭" @click.stop="closeTab(tab.id)">
              <Icon name="X" :size="12" />
            </button>
          </div>
        </div>
        <div class="page-tabs-actions">
          <button type="button" class="page-tabs-more" :aria-expanded="tabMenuOpen" aria-label="标签页操作" title="标签页操作" @click.stop="tabMenuOpen = !tabMenuOpen">
            <Icon name="MoreHorizontal" :size="16" />
          </button>
          <div v-if="tabMenuOpen" class="page-tabs-menu" role="menu">
            <button type="button" role="menuitem" @click="closeOtherTabs"><Icon name="X" :size="14" />关闭其他</button>
            <button type="button" role="menuitem" @click="closeAllTabs"><Icon name="LayoutDashboard" :size="14" />关闭全部</button>
          </div>
        </div>
      </nav>
      <main>
        <div v-if="openTabIds.includes('dashboard')" v-show="page === 'dashboard'" class="page-pane"><Dashboard @navigate="navigate" /></div>
        <div v-if="openTabIds.includes('tasks')" v-show="page === 'tasks'" class="page-pane"><Tasks :global-search="globalSearch" /></div>
        <div v-if="openTabIds.includes('scripts')" v-show="page === 'scripts'" class="page-pane"><Scripts /></div>
        <div v-if="openTabIds.includes('logs')" v-show="page === 'logs'" class="page-pane"><Logs /></div>
        <div v-if="openTabIds.includes('envs')" v-show="page === 'envs'" class="page-pane"><Resources kind="envs" /></div>
        <div v-if="openTabIds.includes('subscriptions')" v-show="page === 'subscriptions'" class="page-pane"><Resources kind="subscriptions" /></div>
        <div v-if="openTabIds.includes('dependencies')" v-show="page === 'dependencies'" class="page-pane"><Resources kind="dependencies" /></div>
        <div v-if="openTabIds.includes('api-docs')" v-show="page === 'api-docs'" class="page-pane"><ApiDocs @navigate="navigate" /></div>
        <div v-if="openTabIds.includes('settings')" v-show="page === 'settings'" class="page-pane"><Settings :version="version" :focus-tab="settingsTab" @consume-tab="settingsTab = ''" /></div>
      </main><footer class="page-footer"><span>大白菜面板 <span class="footer-dot">·</span> 让自动化回归简单</span><span>Go + Vue · SQLite</span></footer>
    </div>
  </div>
  <div v-if="noticeOpen" class="overlay notice-overlay" @mousedown.self="noticeOpen = false"><section class="modal notice-modal" role="dialog" aria-modal="true" aria-labelledby="notice-title"><header><div><span class="eyebrow">NOTICE</span><h2 id="notice-title">通知</h2></div><button class="icon-button" aria-label="关闭通知" @click="noticeOpen = false"><Icon name="X" /></button></header><div class="notice-body"><p v-if="noticeLoading" class="muted">正在获取通知…</p><p v-else-if="noticeText" class="notice-text">{{ noticeText }}</p><p v-else class="muted">暂无通知内容。</p></div><footer><a class="small-button" href="https://github.com/bucai2417/cabbage-panel" target="_blank" rel="noopener noreferrer"><Icon name="GitBranch" :size="15" />项目主页</a><button class="button" @click="noticeOpen = false">关闭</button></footer></section></div>
  <div class="toast-stack" aria-live="polite"><div v-for="notice in notices" :key="notice.id" class="toast" :class="notice.type"><Icon :name="notice.type === 'error' ? 'CircleAlert' : 'CircleCheck'" /><span>{{ notice.message }}</span><button class="icon-button" aria-label="关闭提示" @click="notices = notices.filter(item => item.id !== notice.id)"><Icon name="X" :size="14" /></button></div></div>
  <ConfirmDialog />
</template>
