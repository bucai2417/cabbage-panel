<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'
import { dialogState, settleDialog } from '../dialogs'

const container = ref(null)
const cancelButton = ref(null)
const confirmButton = ref(null)
let previous = null

const actionClass = computed(() => dialogState.tone === 'danger' ? 'danger' : dialogState.tone === 'warning' ? 'warning' : 'primary')

function buttons() {
  return [...(container.value?.querySelectorAll('button') || [])]
}

function keydown(event) {
  if (!dialogState.open) return
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    settleDialog(false)
    return
  }
  if (event.key !== 'Tab') return
  const list = buttons()
  if (!list.length) return
  const first = list[0], last = list[list.length - 1]
  if (!container.value?.contains(document.activeElement)) {
    event.preventDefault()
    first.focus()
  } else if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

watch(() => dialogState.open, async open => {
  if (open) {
    previous = document.activeElement
    document.body.classList.add('confirm-open')
    await nextTick()
    ;(dialogState.tone === 'danger' ? cancelButton.value : confirmButton.value)?.focus()
  } else {
    document.body.classList.remove('confirm-open')
    if (previous?.isConnected) previous.focus()
    previous = null
  }
})

onMounted(() => document.addEventListener('keydown', keydown, true))
onBeforeUnmount(() => {
  document.removeEventListener('keydown', keydown, true)
  document.body.classList.remove('confirm-open')
})
</script>
<template>
  <Teleport to="body">
    <Transition name="confirm-pop">
      <div v-if="dialogState.open" class="confirm-overlay" @mousedown.self="settleDialog(false)">
        <section ref="container" class="confirm-dialog" :class="dialogState.tone" role="alertdialog" aria-modal="true" aria-labelledby="confirm-dialog-title" aria-describedby="confirm-dialog-message">
          <div class="confirm-head">
            <span class="confirm-icon"><Icon :name="dialogState.icon" :size="22" /></span>
            <div>
              <span class="eyebrow">{{ dialogState.eyebrow }}</span>
              <h2 id="confirm-dialog-title">{{ dialogState.title }}</h2>
            </div>
          </div>
          <p id="confirm-dialog-message" class="confirm-message">{{ dialogState.message }}</p>
          <ul v-if="dialogState.details.length" class="confirm-details">
            <li v-for="(item, index) in dialogState.details" :key="index">{{ item }}</li>
          </ul>
          <p v-if="dialogState.note" class="confirm-note"><Icon name="Info" :size="14" />{{ dialogState.note }}</p>
          <footer>
            <button ref="cancelButton" type="button" class="button" @click="settleDialog(false)">{{ dialogState.cancelText }}</button>
            <button ref="confirmButton" type="button" class="button" :class="actionClass" @click="settleDialog(true)">{{ dialogState.confirmText }}</button>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>
