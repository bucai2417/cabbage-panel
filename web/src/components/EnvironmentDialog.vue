<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { api, toast, date, copy } from '../api'
import { confirmDelete } from '../dialogs'
import Icon from './Icon.vue'

const emit = defineEmits(['close'])
const items = ref([]), search = ref(''), loading = ref(true), saving = ref(false), editing = ref(null)
const filtered = computed(() => items.value.filter(item => !search.value || JSON.stringify(item).toLowerCase().includes(search.value.toLowerCase())))
async function load() {
  loading.value = true
  try { items.value = await api('/envs') } catch (error) { toast(error.message, 'error') } finally { loading.value = false }
}
function add() { editing.value = { name: '', value: '', remarks: '' } }
function edit(item) { editing.value = { ...item } }
async function save() {
  if (!editing.value || saving.value) return
  saving.value = true
  try {
    const body = { name: editing.value.name, value: editing.value.value, remarks: editing.value.remarks || '' }
    if (editing.value.id) await api('/envs', 'PUT', { id: editing.value.id, ...body })
    else await api('/envs', 'POST', [body])
    editing.value = null
    await load()
    toast('环境变量已保存')
  } catch (error) { toast(error.message, 'error') } finally { saving.value = false }
}
async function remove(item) {
  if (!await confirmDelete({ title: '删除环境变量「' + item.name + '」？', message: '变量会从后续任务环境中移除。', details: [item.name], confirmText: '删除变量' })) return
  try { await api('/envs', 'DELETE', [item.id]); await load(); toast('环境变量已删除') } catch (error) { toast(error.message, 'error') }
}
async function toggle(item) {
  try { await api('/envs/' + (item.status ? 'enable' : 'disable'), 'PUT', [item.id]); await load() } catch (error) { toast(error.message, 'error') }
}
function keydown(event) {
  if (event.key !== 'Escape' || event.defaultPrevented) return
  event.preventDefault()
  event.stopPropagation()
  if (editing.value) editing.value = null
  else emit('close')
}
onMounted(() => { load(); window.addEventListener('keydown', keydown, true) })
onBeforeUnmount(() => window.removeEventListener('keydown', keydown, true))
</script>
<template>
  <Teleport to="body">
    <div class="overlay env-overlay" @mousedown.self="$emit('close')">
      <section class="modal env-dialog" role="dialog" aria-modal="true" aria-labelledby="env-dialog-title">
        <header><div><span class="eyebrow">ENVIRONMENT</span><h2 id="env-dialog-title">环境变量</h2></div><button class="icon-button" aria-label="关闭环境变量" @click="$emit('close')"><Icon name="X" /></button></header>
        <div class="env-dialog-toolbar"><div class="search-input borderless"><Icon name="Search" :size="16" /><input v-model="search" placeholder="搜索变量名称或值…" aria-label="搜索环境变量" /></div><button class="button primary compact" @click="add"><Icon name="Plus" :size="15" />新增变量</button></div>
        <div v-if="editing" class="env-edit-form">
          <div class="env-edit-heading"><strong>{{ editing.id ? '编辑变量' : '新增变量' }}</strong><span class="muted">保存后立即影响后续任务</span></div>
          <div class="env-edit-grid"><label class="field"><span>变量名称</span><input v-model="editing.name" required placeholder="MY_TOKEN" /></label><label class="field"><span>备注</span><input v-model="editing.remarks" placeholder="用途说明" /></label></div>
          <label class="field"><span>变量值</span><textarea v-model="editing.value" rows="3" placeholder="变量值" /></label>
          <div class="actions justify-end"><button class="button" :disabled="saving" @click="editing = null">取消</button><button class="button primary" :disabled="saving" @click="save"><Icon name="Check" :size="15" />{{ saving ? '保存中' : '保存变量' }}</button></div>
        </div>
        <div class="env-dialog-list">
          <div v-for="item in filtered" :key="item.id" class="env-dialog-row">
            <div class="env-dialog-name"><div class="resource-title-line"><strong>{{ item.name }}</strong><span class="record-id">#{{ item.id }}</span></div><small>{{ date(item.updatedAt || item.createdAt) }}</small></div>
            <code class="env-dialog-value" :title="item.value">{{ item.value }}</code>
            <button class="toggle" role="switch" :aria-checked="!item.status" :aria-label="'启用' + item.name" :class="{ on: !item.status }" @click="toggle(item)"><span /></button>
            <div class="actions"><button class="icon-button" aria-label="复制变量值" title="复制值" @click="copy(item.value)"><Icon name="Copy" :size="15" /></button><button class="icon-button" aria-label="编辑变量" title="编辑" @click="edit(item)"><Icon name="Pencil" :size="15" /></button><button class="icon-button danger" aria-label="删除变量" title="删除" @click="remove(item)"><Icon name="Trash2" :size="15" /></button></div>
          </div>
          <p v-if="loading" class="muted env-dialog-empty">正在加载环境变量…</p>
          <p v-else-if="!filtered.length" class="muted env-dialog-empty">暂无环境变量，点击「新增变量」添加。</p>
        </div>
        <footer><span class="muted">变量值在此弹窗中默认可见，请保管好敏感信息。</span><button class="button" @click="$emit('close')">关闭</button></footer>
      </section>
    </div>
  </Teleport>
</template>
