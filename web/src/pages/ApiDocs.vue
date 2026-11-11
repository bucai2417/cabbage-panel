<script setup>
import { ref, reactive, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { apiGroups, methodTone } from '../apidocs'
import { copy, toast } from '../api'
import Icon from '../components/Icon.vue'

defineEmits(['navigate'])

const stateStorage = 'cabbage-apidocs-state'
const tokenStorage = 'cabbage-apidocs-token'
const restored = typeof sessionStorage !== 'undefined' ? readState() : null

const tab = ref(restored?.tab === 'api' ? 'api' : 'open')
const query = ref('')
const token = ref('')
const openIds = ref(Array.isArray(restored?.openIds) ? restored.openIds : [])
const drafts = reactive({})
const rawView = reactive({})

const prefix = computed(() => tab.value === 'open' ? '/open' : '/api')
const origin = computed(() => (typeof location === 'undefined' ? '' : location.origin))
const openCount = computed(() => count('open'))
const apiCount = computed(() => count('api'))

function readState() {
  try { return JSON.parse(sessionStorage.getItem(stateStorage) || 'null') } catch { return null }
}
function persistState() {
  if (typeof sessionStorage === 'undefined') return
  sessionStorage.setItem(stateStorage, JSON.stringify({ tab: tab.value, openIds: openIds.value }))
}

function count(tabId) {
  return apiGroups.reduce((total, group) => total + group.endpoints.filter(ep => ep.tabs.includes(tabId)).length, 0)
}
function visibleIn(group, tabId) {
  return group.endpoints.filter(ep => ep.tabs.includes(tabId) && matches(ep, group))
}
// 搜索同时覆盖分组名与说明，输入「环境」这类分组关键词时也能命中。
function matches(ep, group) {
  const text = query.value.trim().toLowerCase()
  if (!text) return true
  return [ep.method, ep.path, ep.summary, ep.description, ep.scope, group?.label, group?.intro]
    .filter(Boolean).join(' ').toLowerCase().includes(text)
}
const visibleGroups = computed(() => apiGroups
  .map(group => ({ ...group, endpoints: visibleIn(group, tab.value) }))
  .filter(group => group.endpoints.length))

function placeholders(ep) {
  return [...new Set([...ep.path.matchAll(/:([A-Za-z0-9_]+)/g)].map(match => match[1]))]
}
function pathParams(ep) {
  const declared = (ep.params || []).filter(param => param.in === 'path')
  return placeholders(ep).map(name => declared.find(param => param.name === name) || { name, in: 'path', type: 'string', description: '', example: '' })
}
function authLabel(ep) {
  if (ep.noAuth) return '公开'
  return ep.scope || '管理员'
}
function isDestructive(ep) {
  return ep.method === 'DELETE' || /(^|\/)(stop|cancel|reinstall|force|apply|import|init|logout|debug)$/.test(ep.path)
}
function createDraft(ep) {
  const draft = {
    path: {},
    query: (ep.params || []).filter(param => param.in === 'query').map(param => ({ key: param.name, value: param.example || '', description: param.description || '' })),
    body: ep.body ? JSON.stringify(ep.body, null, 2) : '',
    file: null,
    busy: false,
    result: null,
  }
  for (const param of pathParams(ep)) draft.path[param.name] = param.example || ''
  return draft
}
function draftOf(ep) {
  if (!drafts[ep.id]) drafts[ep.id] = createDraft(ep)
  return drafts[ep.id]
}
function resetDraft(ep) {
  drafts[ep.id] = createDraft(ep)
  toast('已恢复示例值')
}
function isOpen(ep) { return openIds.value.includes(ep.id) }
function toggle(ep) {
  if (isOpen(ep)) { openIds.value = openIds.value.filter(id => id !== ep.id); persistState(); return }
  draftOf(ep)
  openIds.value = [...openIds.value, ep.id]
  persistState()
}
function expandAll() { openIds.value = visibleGroups.value.flatMap(group => group.endpoints.map(ep => { draftOf(ep); return ep.id })); persistState() }
function collapseAll() { openIds.value = []; persistState() }
function scrollTo(id) {
  document.getElementById('apidocs-' + id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  activeGroup.value = id
}
const activeGroup = ref(visibleGroups.value[0]?.id || '')
// 滚动时同步高亮左侧分组，避免长列表里丢失位置。
function trackActiveGroup() {
  const marks = visibleGroups.value
    .map(group => ({ id: group.id, el: document.getElementById('apidocs-' + group.id) }))
    .filter(item => item.el)
  if (!marks.length) return
  const line = 140
  let current = marks[0].id
  for (const mark of marks) {
    if (mark.el.getBoundingClientRect().top <= line) current = mark.id
  }
  activeGroup.value = current
}
function addQuery(draft) { draft.query.push({ key: '', value: '', description: '' }) }
function removeQuery(draft, index) { draft.query.splice(index, 1) }
function resetQuery(draft, ep) {
  draft.query = (ep.params || []).filter(param => param.in === 'query').map(param => ({ key: param.name, value: param.example || '', description: param.description || '' }))
}

function buildUrl(ep, draft, tolerant = false) {
  let path = ep.path
  for (const param of pathParams(ep)) {
    const value = String(draft.path[param.name] ?? '').trim()
    if (!value) {
      if (tolerant) continue
      throw new Error('请填写路径参数「' + (param.description || param.name) + '」')
    }
    path = path.replace(new RegExp(':' + param.name + '(?![A-Za-z0-9_])', 'g'), encodeURIComponent(value))
  }
  if (!tolerant) {
    const left = path.match(/:[A-Za-z0-9_]+/g)
    if (left) throw new Error('请填写路径参数 ' + left.join('、'))
  }
  const base = ep.absolute ? path : prefix.value + path
  const url = new URL(base, location.origin)
  for (const item of draft.query) {
    const key = String(item.key || '').trim()
    if (!key || String(item.value).trim() === '') continue
    url.searchParams.set(key, item.value)
  }
  return url
}

// 预览最终请求地址；路径参数未填时显示占位提示，而不是报错。
function previewUrl(ep) {
  const draft = draftOf(ep)
  try {
    return buildUrl(ep, draft, true).toString().replace(origin.value, '')
  } catch {
    return (ep.absolute ? ep.path : prefix.value + ep.path)
  }
}

async function send(ep, draft) {
  if (ep.streaming || draft.busy) return
  draft.busy = true
  draft.result = null
  const started = performance.now()
  try {
    const url = buildUrl(ep, draft)
    const init = { method: ep.method, credentials: 'same-origin', cache: 'no-store', headers: {} }
    if (token.value.trim() && !ep.noAuth) init.headers.Authorization = 'Bearer ' + token.value.trim()
    if (ep.upload) {
      if (!draft.file) throw new Error('请选择要上传的文件')
      const form = new FormData()
      form.append('file', draft.file)
      init.body = form
    } else if (ep.body && !['GET', 'HEAD'].includes(ep.method)) {
      let parsed
      try { parsed = JSON.parse(draft.body) } catch { throw new Error('请求体不是合法 JSON，请修正后再发送') }
      init.headers['Content-Type'] = 'application/json'
      init.body = JSON.stringify(parsed)
    }
    const response = await fetch(url, init)
    const type = (response.headers.get('content-type') || '').toLowerCase()
    let body = '', binary = false
    if (type.includes('application/json')) {
      const text = await response.text()
      try { body = JSON.stringify(JSON.parse(text), null, 2) } catch { body = text }
    } else if (type.startsWith('text/') || type.includes('xml') || type === '') {
      body = await response.text()
    } else {
      const blob = await response.blob()
      binary = true
      body = '[' + (blob.type || '二进制') + ' 响应 · ' + blob.size + ' 字节]\n已跳过内容展示，可在浏览器中直接打开该地址下载。'
    }
    rawView[ep.id] = false
    draft.result = {
      ok: response.ok,
      status: response.status,
      statusText: response.statusText || '',
      ms: Math.round(performance.now() - started),
      bytes: new Blob([body]).size,
      body, raw: body, binary,
      url: url.toString(),
    }
  } catch (error) {
    draft.result = { error: error.message || '请求失败', ms: Math.round(performance.now() - started) }
  } finally {
    draft.busy = false
  }
}

function responseText(ep) {
  const result = draftOf(ep).result
  if (!result) return ''
  return result.error || result.body
}
function toggleRaw(ep) { rawView[ep.id] = !rawView[ep.id] }
function prettyView(ep) {
  const result = draftOf(ep).result
  if (!result || rawView[ep.id] || result.error) return responseText(ep)
  return result.body
}

function curlFor(ep, draft) {
  let url
  try { url = buildUrl(ep, draft, true).toString() } catch { url = origin.value + (ep.absolute ? ep.path : prefix.value + ep.path) }
  const lines = [`curl -X ${ep.method} '${url}'`]
  if (!ep.noAuth) lines.push("  -H 'Authorization: Bearer $CABBAGE_TOKEN'")
  if (ep.upload) lines.push("  -F 'file=@backup.zip'")
  else if (ep.body && !['GET', 'HEAD'].includes(ep.method)) {
    let payload = draft.body.trim()
    try { payload = JSON.stringify(JSON.parse(payload)) } catch { /* 保留原始输入 */ }
    lines.push("  -H 'Content-Type: application/json'")
    lines.push(`  -d '${payload.replace(/'/g, "'\\''")}'`)
  }
  return lines.join(' \\\n')
}

function clearToken() { token.value = ''; toast('已清除 Authorization') }
watch(token, value => {
  if (typeof sessionStorage === 'undefined') return
  if (value) sessionStorage.setItem(tokenStorage, value)
  else sessionStorage.removeItem(tokenStorage)
})
watch(tab, () => { persistState(); nextTick(() => { activeGroup.value = visibleGroups.value[0]?.id || ''; trackActiveGroup() }) })
onMounted(() => {
  if (typeof sessionStorage !== 'undefined') token.value = sessionStorage.getItem(tokenStorage) || ''
  for (const id of openIds.value) {
    for (const group of apiGroups) {
      const ep = group.endpoints.find(item => item.id === id)
      if (ep) draftOf(ep)
    }
  }
  activeGroup.value = visibleGroups.value[0]?.id || ''
  window.addEventListener('scroll', trackActiveGroup, { passive: true })
})
onUnmounted(() => window.removeEventListener('scroll', trackActiveGroup))
</script>

<template>
  <section class="apidocs-hero">
    <div class="apidocs-hero-main">
      <span class="eyebrow">DEVELOPER · HTTP API</span>
      <h1><Icon name="BookOpen" :size="26" />接口文档</h1>
      <p>开放 API 与内置 API 使用同一套资源路由，区别是前缀与权限：<code>/open</code> 面向开放应用，按应用 scope 授权；<code>/api</code> 面向面板自身，包含仅管理员可用的系统、账号、应用与备份接口。</p>
    </div>
    <div class="apidocs-hero-side">
      <span class="badge"><i />{{ origin }}</span>
      <span class="badge success"><i />当前前缀 {{ prefix }}</span>
    </div>
  </section>

  <div class="apidocs-toolbar">
    <div class="tabs">
      <button :class="{ active: tab === 'open' }" @click="tab = 'open'">开放 API<span>{{ openCount }}</span></button>
      <button :class="{ active: tab === 'api' }" @click="tab = 'api'">内置 API<span>{{ apiCount }}</span></button>
    </div>
    <div class="search-input apidocs-search"><Icon name="Search" /><input v-model="query" placeholder="搜索路径、名称或权限…" aria-label="搜索接口" /></div>
    <div class="actions">
      <button class="small-button" @click="expandAll">全部展开</button>
      <button class="small-button" @click="collapseAll">全部折叠</button>
    </div>
  </div>

  <div class="info-banner apidocs-banner">
    <Icon name="Info" />
    <span v-if="tab === 'open'">开放应用接口需要 <code>Authorization: Bearer &lt;token&gt;</code>，权限由应用 scope 决定。先在「系统设置 → 开放应用 / MCP」创建应用并生成 Authorization，或先用下方的 <code>/open/auth/token</code> 换取令牌。未填写令牌时，将以当前登录的管理员会话发起请求，仅便于在本页调试。</span>
    <span v-else>内置接口使用当前登录会话（同源 HttpOnly Cookie）。填写下方 Authorization 可覆盖会话身份，用于验证应用权限。系统、账号、开放应用与备份接口仅管理员可用。</span>
  </div>

  <div class="apidocs-auth">
    <label for="apidocs-token"><Icon name="KeyRound" :size="16" />Authorization</label>
    <input id="apidocs-token" v-model="token" type="password" autocomplete="off" spellcheck="false" placeholder="Bearer token，留空则使用当前登录会话" />
    <button class="small-button" @click="clearToken">清除</button>
    <span class="apidocs-auth-hint">仅保存在当前标签页的 sessionStorage，关闭页面即清除。</span>
  </div>

  <div class="apidocs-layout">
    <aside class="apidocs-nav" aria-label="接口分组">
      <button v-for="group in visibleGroups" :key="group.id" :class="{ active: activeGroup === group.id }" @click="scrollTo(group.id)">
        <Icon :name="group.icon" :size="16" />
        <span>{{ group.label }}</span>
        <em>{{ group.endpoints.length }}</em>
      </button>
      <p v-if="!visibleGroups.length" class="apidocs-nav-empty">没有匹配的接口</p>
    </aside>

    <div class="apidocs-body">
      <section v-for="group in visibleGroups" :id="'apidocs-' + group.id" :key="group.id" class="apidocs-group">
        <header class="apidocs-group-head">
          <h2><span class="apidocs-group-icon"><Icon :name="group.icon" :size="18" /></span>{{ group.label }}</h2>
          <span class="muted">{{ group.endpoints.length }} 个接口</span>
        </header>
        <p v-if="group.intro" class="apidocs-group-intro">{{ group.intro }}</p>

        <article v-for="ep in group.endpoints" :key="ep.id" class="endpoint" :class="{ open: isOpen(ep) }">
          <button class="endpoint-head" :aria-expanded="isOpen(ep)" @click="toggle(ep)">
            <span class="method" :class="methodTone[ep.method]">{{ ep.method }}</span>
            <code class="endpoint-path">{{ ep.path }}</code>
            <span class="endpoint-summary">{{ ep.summary }}</span>
            <span class="endpoint-scope" :class="{ public: ep.noAuth }">{{ authLabel(ep) }}</span>
            <Icon name="ChevronDown" :size="16" class="endpoint-chevron" />
          </button>

          <div v-if="isOpen(ep)" class="endpoint-body">
            <p class="endpoint-desc">{{ ep.description }}</p>
            <p v-if="isDestructive(ep)" class="apidocs-warn"><Icon name="CircleAlert" :size="14" />这是写操作或会产生实际影响，发送前请确认参数。</p>

            <div class="apidocs-preview">
              <span class="apidocs-preview-label">请求地址</span>
              <code>{{ ep.method }} {{ previewUrl(ep) }}</code>
            </div>

            <div v-if="pathParams(ep).length" class="apidocs-block">
              <h4>路径参数</h4>
              <div class="apidocs-fields">
                <label v-for="param in pathParams(ep)" :key="param.name" class="apidocs-field">
                  <span><code>{{ param.name }}</code><em v-if="param.required !== false">必填</em></span>
                  <select v-if="param.options" v-model="draftOf(ep).path[param.name]" :aria-label="param.name">
                    <option v-for="option in param.options" :key="option" :value="option">{{ option }}</option>
                  </select>
                  <input v-else v-model="draftOf(ep).path[param.name]" :placeholder="param.example || param.name" :aria-label="param.name" spellcheck="false" />
                  <small v-if="param.description">{{ param.description }}</small>
                </label>
              </div>
            </div>

            <div class="apidocs-block">
              <h4>查询参数 <button class="text-button" @click="addQuery(draftOf(ep))"><Icon name="Plus" :size="14" />添加参数</button></h4>
              <div v-if="draftOf(ep).query.length" class="apidocs-query">
                <div class="apidocs-query-row apidocs-query-labels"><span>参数名</span><span>参数值</span><span>说明</span><span /></div>
                <div v-for="(item, index) in draftOf(ep).query" :key="index" class="apidocs-query-row">
                  <input v-model="item.key" placeholder="参数名" aria-label="查询参数名" spellcheck="false" />
                  <input v-model="item.value" placeholder="参数值" aria-label="查询参数值" spellcheck="false" />
                  <span class="apidocs-query-hint">{{ item.description || '—' }}</span>
                  <button class="icon-button danger" aria-label="移除参数" title="移除" @click="removeQuery(draftOf(ep), index)"><Icon name="X" :size="15" /></button>
                </div>
                <button class="text-button apidocs-query-reset" @click="resetQuery(draftOf(ep), ep)"><Icon name="RefreshCw" :size="13" />恢复默认参数</button>
              </div>
              <p v-else class="muted apidocs-none">此接口没有预置查询参数，可按需添加。</p>
            </div>

            <div v-if="ep.body" class="apidocs-block">
              <h4>请求体 <span class="muted">JSON</span><button class="text-button" @click="draftOf(ep).body = JSON.stringify(ep.body, null, 2)"><Icon name="RefreshCw" :size="13" />恢复示例</button></h4>
              <textarea v-model="draftOf(ep).body" class="apidocs-editor" rows="10" spellcheck="false" aria-label="请求体 JSON" />
            </div>

            <div v-if="ep.upload" class="apidocs-block">
              <h4>上传文件</h4>
              <input type="file" @change="draftOf(ep).file = $event.target.files[0]" />
              <small class="muted">以 multipart/form-data 的 file 字段提交。</small>
            </div>

            <div class="apidocs-actions">
              <button class="button primary compact" :disabled="draftOf(ep).busy || ep.streaming" @click="send(ep, draftOf(ep))">
                <Icon :name="draftOf(ep).busy ? 'LoaderCircle' : 'Send'" :size="15" :class="{ spin: draftOf(ep).busy }" />
                {{ ep.streaming ? '流式接口请在代码中调用' : draftOf(ep).busy ? '请求中…' : '发送请求' }}
              </button>
              <button class="button compact" @click="resetDraft(ep)"><Icon name="RefreshCw" :size="15" />重置示例</button>
              <button class="button compact" @click="copy(curlFor(ep, draftOf(ep)))"><Icon name="Copy" :size="15" />复制 cURL</button>
              <span v-if="ep.streaming" class="apidocs-inline-note">该接口为 SSE 流，浏览器内不便展示，请用 EventSource 或 curl 调用。</span>
            </div>

            <div v-if="draftOf(ep).result" class="apidocs-result" :class="draftOf(ep).result.error ? 'err' : draftOf(ep).result.ok ? 'ok' : 'warn'">
              <div class="apidocs-result-head">
                <span class="status-pill">{{ draftOf(ep).result.error ? '请求失败' : draftOf(ep).result.status + ' ' + draftOf(ep).result.statusText }}</span>
                <span class="apidocs-result-meta">{{ draftOf(ep).result.ms }} ms<template v-if="draftOf(ep).result.bytes"> · {{ draftOf(ep).result.bytes }} B</template></span>
                <button v-if="!draftOf(ep).result.error && !draftOf(ep).result.binary" class="text-button" @click="toggleRaw(ep)">{{ rawView[ep.id] ? '格式化' : '原文' }}</button>
                <button class="text-button" @click="copy(responseText(ep))"><Icon name="Copy" :size="14" />复制响应</button>
              </div>
              <pre class="apidocs-result-body">{{ prettyView(ep) }}</pre>
            </div>

            <details v-if="ep.response" class="apidocs-sample">
              <summary>示例响应</summary>
              <pre>{{ JSON.stringify(ep.response, null, 2) }}</pre>
            </details>
          </div>
        </article>
      </section>

      <div v-if="!visibleGroups.length" class="empty-state apidocs-empty">
        <div class="empty-icon"><Icon name="Search" :size="30" /></div>
        <h3>没有匹配的接口</h3>
        <p>试试其他关键词，或切换开放 API / 内置 API。</p>
      </div>
    </div>
  </div>
</template>
