<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import Icon from './Icon.vue'
const props = defineProps({ title: String, fields: Array, initial: Object, busy: Boolean, wide: Boolean })
const emit = defineEmits(['close', 'submit'])
const model = ref(JSON.parse(JSON.stringify(props.initial || {})))
const container = ref(null)
let previous
function keydown(event) {
  if (event.key === 'Escape' && !props.busy) emit('close')
  if (event.key === 'Tab') {
    const elements = container.value?.querySelectorAll('button:not(:disabled), input, select, textarea')
    if (!elements?.length) return
    if (event.shiftKey && document.activeElement === elements[0]) { event.preventDefault(); elements[elements.length - 1].focus() }
    else if (!event.shiftKey && document.activeElement === elements[elements.length - 1]) { event.preventDefault(); elements[0].focus() }
  }
}
onMounted(async () => { previous = document.activeElement; document.addEventListener('keydown', keydown); await nextTick(); container.value?.querySelector('input, textarea, select, button')?.focus() })
onUnmounted(() => { document.removeEventListener('keydown', keydown); previous?.focus() })
</script>
<template>
  <Teleport to="body"><div class="overlay" @mousedown.self="!busy && emit('close')"><section ref="container" class="modal" :class="{ wide }" role="dialog" aria-modal="true" aria-labelledby="dialog-title">
    <header><div><span class="eyebrow">CABBAGE PANEL</span><h2 id="dialog-title">{{ title }}</h2></div><button class="icon-button" aria-label="关闭" :disabled="busy" @click="emit('close')"><Icon name="X" /></button></header>
    <form @submit.prevent="emit('submit', model)">
      <div class="form-body"><label v-for="field in fields" :key="field.key" class="field" :class="{ checkbox: field.type === 'checkbox' }">
        <span>{{ field.label }} <i v-if="field.required">*</i></span>
        <textarea v-if="field.type === 'textarea'" v-model="model[field.key]" :aria-label="field.label" :rows="field.rows || 4" :required="field.required" :placeholder="field.placeholder" :class="{ code: field.code }" />
        <select v-else-if="field.type === 'select'" v-model="model[field.key]" :aria-label="field.label"><option v-for="option in field.options" :key="option.value" :value="option.value">{{ option.label }}</option></select>
        <input v-else-if="field.type === 'checkbox'" v-model="model[field.key]" :aria-label="field.label" type="checkbox" />
        <input v-else-if="field.type === 'file'" :aria-label="field.label" type="file" :required="field.required" @change="model[field.key] = $event.target.files[0]" />
        <input v-else v-model="model[field.key]" :aria-label="field.label" :type="field.type || 'text'" :required="field.required" :placeholder="field.placeholder" :min="field.min" :max="field.max" :minlength="field.minlength" :autocomplete="field.type === 'password' ? 'new-password' : 'off'" />
        <small v-if="field.hint">{{ field.hint }}</small>
      </label><slot /></div>
      <footer><button type="button" class="button" :disabled="busy" @click="emit('close')">取消</button><button class="button primary" :disabled="busy"><Icon :name="busy ? 'LoaderCircle' : 'Check'" :class="{ spin: busy }" />{{ busy ? '正在保存' : '保存' }}</button></footer>
    </form>
  </section></div></Teleport>
</template>
